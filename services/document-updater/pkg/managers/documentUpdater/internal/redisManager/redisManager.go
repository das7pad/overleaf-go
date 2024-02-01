// Golang port of Overleaf
// Copyright (C) 2021-2024 Jakob Ackermann <das7pad@outlook.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package redisManager

import (
	"context"
	"encoding/json"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"

	"github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
)

type Manager interface {
	PutDocInMemory(ctx context.Context, projectId sharedTypes.UUID, docId sharedTypes.UUID, doc *types.Doc) error
	RemoveDocFromMemory(ctx context.Context, projectId sharedTypes.UUID, docId sharedTypes.UUID) error
	RemoveDocFromProject(ctx context.Context, projectId, docId sharedTypes.UUID) error
	GetDoc(ctx context.Context, projectId sharedTypes.UUID, docId sharedTypes.UUID) (*types.Doc, error)
	GetDocVersion(ctx context.Context, docId sharedTypes.UUID) (sharedTypes.Version, error)
	GetPreviousDocUpdates(ctx context.Context, docId sharedTypes.UUID, start sharedTypes.Version, end sharedTypes.Version) ([]sharedTypes.DocumentUpdate, error)
	GetPreviousDocUpdatesUnderLock(ctx context.Context, docId sharedTypes.UUID, begin sharedTypes.Version, end sharedTypes.Version, docVersion sharedTypes.Version) ([]sharedTypes.DocumentUpdate, error)
	UpdateDocument(ctx context.Context, docId sharedTypes.UUID, doc *types.Doc, appliedUpdates []sharedTypes.DocumentUpdate) (int64, error)
	RenameDoc(ctx context.Context, projectId sharedTypes.UUID, docId sharedTypes.UUID, doc *types.Doc, newPath sharedTypes.PathName) error
	ClearUnFlushedTime(ctx context.Context, docId sharedTypes.UUID) error
	GetDocIdsInProject(ctx context.Context, projectId sharedTypes.UUID) ([]sharedTypes.UUID, error)
	QueueFlushAndDeleteProject(ctx context.Context, projectId sharedTypes.UUID) error
	GetNextProjectToFlushAndDelete(ctx context.Context, cutoffTime time.Time) (sharedTypes.UUID, int64, int64, error)
}

func New(rClient redis.UniversalClient) Manager {
	return &manager{rClient: rClient}
}

const (
	DocOpsTTL       = 60 * time.Minute
	DocOpsMaxLength = 100
)

var ErrUpdateRangeNotAvailable = &errors.UpdateRangeNotAvailableError{}

type manager struct {
	rClient redis.UniversalClient
}

func getDocsInProjectKey(projectId sharedTypes.UUID) string {
	b := make([]byte, 0, 8+36+1)
	b = append(b, "DocsIn:{"...)
	b = projectId.Append(b)
	b = append(b, '}')
	return string(b)
}

func getDocCoreKey(docId sharedTypes.UUID) string {
	b := make([]byte, 0, 9+36+1)
	b = append(b, "docCore:{"...)
	b = docId.Append(b)
	b = append(b, '}')
	return string(b)
}

func getDocVersionKey(docId sharedTypes.UUID) string {
	b := make([]byte, 0, 12+36+1)
	b = append(b, "DocVersion:{"...)
	b = docId.Append(b)
	b = append(b, '}')
	return string(b)
}

func getUnFlushedTimeKey(docId sharedTypes.UUID) string {
	b := make([]byte, 0, 15+36+1)
	//goland:noinspection SpellCheckingInspection
	b = append(b, "UnflushedTime:{"...)
	b = docId.Append(b)
	b = append(b, '}')
	return string(b)
}

func getLastUpdatedCtxKey(docId sharedTypes.UUID) string {
	b := make([]byte, 0, 16+36+1)
	b = append(b, "lastUpdatedCtx:{"...)
	b = docId.Append(b)
	b = append(b, '}')
	return string(b)
}

func getDocUpdatesKey(docId sharedTypes.UUID) string {
	b := make([]byte, 0, 8+36+1)
	b = append(b, "DocOps:{"...)
	b = docId.Append(b)
	b = append(b, '}')
	return string(b)
}

func getUncompressedHistoryOpsKey(docId sharedTypes.UUID) string {
	b := make([]byte, 0, 24+36+1)
	b = append(b, "UncompressedHistoryOps:{"...)
	b = docId.Append(b)
	b = append(b, '}')
	return string(b)
}

func getFlushAndDeleteQueueKey() string {
	return "DocUpdaterFlushAndDeleteQueue"
}

func (m *manager) PutDocInMemory(ctx context.Context, projectId sharedTypes.UUID, docId sharedTypes.UUID, doc *types.Doc) error {
	err := m.rClient.SAdd(ctx, getDocsInProjectKey(projectId), docId.String()).Err()
	if err != nil {
		return errors.Tag(err, "record doc in project")
	}
	coreBlob, err := doc.DocCore.DoMarshalJSON()
	if err != nil {
		return errors.Tag(err, "serialize DocCore")
	}
	vars := map[string]interface{}{
		getDocCoreKey(docId):    coreBlob,
		getDocVersionKey(docId): doc.Version.String(),
	}
	if doc.UnFlushedTime != 0 {
		vars[getUnFlushedTimeKey(docId)] = int64(doc.UnFlushedTime)
	}
	if err = m.rClient.MSet(ctx, vars).Err(); err != nil {
		return errors.Tag(err, "persist in redis")
	}
	return nil
}

func (m *manager) RemoveDocFromMemory(ctx context.Context, projectId sharedTypes.UUID, docId sharedTypes.UUID) error {
	err := m.rClient.Del(
		ctx,
		getDocCoreKey(docId),
		getDocVersionKey(docId),
		getUnFlushedTimeKey(docId),
		getLastUpdatedCtxKey(docId),
	).Err()
	if err != nil {
		return errors.Tag(err, "cleanup doc details")
	}
	return m.RemoveDocFromProject(ctx, projectId, docId)
}

func (m *manager) RemoveDocFromProject(ctx context.Context, projectId, docId sharedTypes.UUID) error {
	key := getDocsInProjectKey(projectId)
	if err := m.rClient.SRem(ctx, key, docId.String()).Err(); err != nil {
		return errors.Tag(err, "cleanup project tracking")
	}
	return nil
}

func (m *manager) GetDoc(ctx context.Context, projectId sharedTypes.UUID, docId sharedTypes.UUID) (*types.Doc, error) {
	res := m.rClient.MGet(
		ctx,
		getDocCoreKey(docId),
		getDocVersionKey(docId),
		getUnFlushedTimeKey(docId),
		getLastUpdatedCtxKey(docId),
	)
	if err := res.Err(); err != nil {
		return nil, errors.Tag(err, "get doc details from redis")
	}
	results := res.Val()
	if len(results) != 4 {
		return nil, errors.New("too few values returned from redis")
	}
	if results[0] == "" || results[0] == nil {
		return nil, &errors.NotFoundError{}
	}
	blobs := make([][]byte, len(results))
	for i, result := range results {
		switch value := result.(type) {
		case []byte:
			blobs[i] = value
		case string:
			blobs[i] = []byte(value)
		case nil:
			blobs[i] = nil
		default:
			return nil, errors.New("unexpected value from redis")
		}
	}
	doc := types.Doc{
		DocId: docId,
	}
	if err := doc.DocCore.DoUnmarshalJSON(blobs[0]); err != nil {
		return nil, errors.Tag(err, "parse doc core")
	}
	if doc.ProjectId != projectId {
		return nil, &errors.NotAuthorizedError{}
	}

	if err := json.Unmarshal(blobs[1], &doc.Version); err != nil {
		return nil, errors.Tag(err, "parse doc version")
	}
	if len(blobs[2]) != 0 {
		if err := json.Unmarshal(blobs[2], &doc.UnFlushedTime); err != nil {
			return nil, errors.Tag(err, "parse doc un-flushed time")
		}
	}
	if len(blobs[3]) > 2 {
		if err := json.Unmarshal(blobs[3], &doc.LastUpdatedCtx); err != nil {
			return nil, errors.Tag(err, "parse last updated ctx")
		}
	}
	return &doc, nil
}

func (m *manager) GetDocVersion(ctx context.Context, docId sharedTypes.UUID) (sharedTypes.Version, error) {
	v, err := m.rClient.Get(ctx, getDocVersionKey(docId)).Int64()
	if err != nil {
		if err == redis.Nil {
			err = &errors.NotFoundError{}
		}
		return 0, errors.Tag(err, "get version from redis")
	}
	return sharedTypes.Version(v), nil
}

var scriptGetPreviousDocUpdates = redis.NewScript(`
local length = redis.call("LLEN", KEYS[1])
if length == 0 then error("overleaf: length is 0") end

local version = tonumber(redis.call("GET", KEYS[2]), 10)
if version == nil then error("overleaf: version not found") end

local first_version_in_redis = version - length
local start = tonumber(ARGV[1], 10)
local stop = tonumber(ARGV[2], 10)

if start < first_version_in_redis then error("overleaf: too old start") end
if stop > version then error("overleaf: end in future") end

start = start - first_version_in_redis
if stop > -1 then stop = (stop - first_version_in_redis) end

return redis.call("LRANGE", KEYS[1], start, stop)
`)

func (m *manager) GetPreviousDocUpdates(ctx context.Context, docId sharedTypes.UUID, start sharedTypes.Version, end sharedTypes.Version) ([]sharedTypes.DocumentUpdate, error) {
	if start == end {
		return nil, nil
	}
	keys := []string{
		getDocUpdatesKey(docId),
		getDocVersionKey(docId),
	}
	blobs, err := scriptGetPreviousDocUpdates.
		Run(ctx, m.rClient, keys, start.String(), end.String()).
		StringSlice()
	if err != nil {
		if strings.Contains(err.Error(), "overleaf:") {
			return nil, ErrUpdateRangeNotAvailable
		}
		return nil, errors.Tag(err, "get previous updates from redis")
	}
	return m.parseDocumentUpdates(start, blobs)
}

func (m *manager) GetPreviousDocUpdatesUnderLock(ctx context.Context, docId sharedTypes.UUID, begin sharedTypes.Version, end sharedTypes.Version, docVersion sharedTypes.Version) ([]sharedTypes.DocumentUpdate, error) {
	if begin == end {
		return nil, nil
	}
	n := int64(end - begin)
	offset := int64(docVersion - end)
	start := -n - offset
	stop := -1 - offset
	raw, err := m.rClient.LRange(
		ctx, getDocUpdatesKey(docId), start, stop,
	).Result()
	if err != nil {
		return nil, errors.Tag(err, "get previous updates from redis")
	}
	if len(raw) != int(n) {
		return nil, ErrUpdateRangeNotAvailable
	}
	return m.parseDocumentUpdates(begin, raw)
}

func (m *manager) parseDocumentUpdates(start sharedTypes.Version, raw []string) ([]sharedTypes.DocumentUpdate, error) {
	updates := make([]sharedTypes.DocumentUpdate, len(raw))
	for i, s := range raw {
		if err := json.Unmarshal([]byte(s), &updates[i]); err != nil {
			return nil, errors.Tag(err, "parse update")
		}
		if i == 0 && start != updates[i].Version {
			return nil, ErrUpdateRangeNotAvailable
		}
	}
	return updates, nil
}

func (m *manager) UpdateDocument(ctx context.Context, docId sharedTypes.UUID, doc *types.Doc, appliedUpdates []sharedTypes.DocumentUpdate) (int64, error) {
	currentVersion, err := m.GetDocVersion(ctx, docId)
	if err != nil {
		return 0, errors.Tag(err, "get doc version for validation")
	}
	nUpdatesOffset := sharedTypes.Version(len(appliedUpdates))
	if currentVersion != doc.Version-nUpdatesOffset {
		return 0, errors.New(
			"refusing to update: remote version mismatches local version: " +
				currentVersion.String() +
				" != " +
				doc.Version.String() +
				" - " +
				nUpdatesOffset.String(),
		)
	}

	coreBlob, err := doc.DocCore.DoMarshalJSON()
	if err != nil {
		return 0, errors.Tag(err, "serialize doc core")
	}
	appliedUpdatesBlobs := make([]interface{}, len(appliedUpdates))
	for i, update := range appliedUpdates {
		appliedUpdateBlob, err2 := json.Marshal(update)
		if err2 != nil {
			return 0, errors.Tag(err2, "serialize applied update")
		}
		appliedUpdatesBlobs[i] = appliedUpdateBlob
	}
	lastUpdatedCtxBlob, err := json.Marshal(doc.LastUpdatedCtx)
	if err != nil {
		return 0, errors.Tag(err, "serialize last updated ctx")
	}
	var uncompressedHistoryOpsRes *redis.IntCmd
	_, err = m.rClient.TxPipelined(ctx, func(p redis.Pipeliner) error {
		p.MSet(ctx, map[string]interface{}{
			getDocCoreKey(docId):        coreBlob,
			getDocVersionKey(docId):     doc.Version.String(),
			getLastUpdatedCtxKey(docId): lastUpdatedCtxBlob,
		})
		p.LTrim(ctx, getDocUpdatesKey(docId), -DocOpsMaxLength, -1)
		p.RPush(ctx, getDocUpdatesKey(docId), appliedUpdatesBlobs...)
		p.Expire(ctx, getDocUpdatesKey(docId), DocOpsTTL)

		uncompressedHistoryOpsRes = p.RPush(
			ctx,
			getUncompressedHistoryOpsKey(docId),
			appliedUpdatesBlobs...,
		)
		now := time.Now().Unix()
		doc.UnFlushedTime = types.UnFlushedTime(now)
		p.SetNX(ctx, getUnFlushedTimeKey(docId), now, 0)
		return nil
	})
	if err != nil {
		return 0, errors.Tag(err, "update doc in redis")
	}
	if uncompressedHistoryOpsRes != nil {
		return uncompressedHistoryOpsRes.Val(), nil
	}
	return -1, nil
}

func (m *manager) RenameDoc(ctx context.Context, projectId sharedTypes.UUID, docId sharedTypes.UUID, doc *types.Doc, newPath sharedTypes.PathName) error {
	doc.PathName = newPath
	if err := m.PutDocInMemory(ctx, projectId, docId, doc); err != nil {
		return errors.Tag(err, "rewrite doc in redis")
	}
	return nil
}

func (m *manager) ClearUnFlushedTime(ctx context.Context, docId sharedTypes.UUID) error {
	return m.rClient.Del(ctx, getUnFlushedTimeKey(docId)).Err()
}

func (m *manager) GetDocIdsInProject(ctx context.Context, projectId sharedTypes.UUID) ([]sharedTypes.UUID, error) {
	res := m.rClient.SMembers(ctx, getDocsInProjectKey(projectId))
	if err := res.Err(); err != nil {
		return nil, errors.Tag(err, "get docs from redis")
	}
	rawIds := res.Val()
	docIds := make([]sharedTypes.UUID, len(rawIds))
	for i, raw := range rawIds {
		id, err := sharedTypes.ParseUUID(raw)
		if err != nil {
			return nil, errors.Tag(err, "parse raw docId: "+raw)
		}
		docIds[i] = id
	}
	return docIds, nil
}

const SmoothingOffset = int64(time.Second)

func (m *manager) QueueFlushAndDeleteProject(ctx context.Context, projectId sharedTypes.UUID) error {
	smoothingOffset := time.Duration(rand.Int63n(SmoothingOffset))
	score := time.Now().Add(smoothingOffset).Unix()
	queueEntry := redis.Z{
		Score:  float64(score),
		Member: projectId.String(),
	}
	return m.rClient.ZAdd(ctx, getFlushAndDeleteQueueKey(), &queueEntry).Err()
}

func (m *manager) GetNextProjectToFlushAndDelete(ctx context.Context, cutoffTime time.Time) (sharedTypes.UUID, int64, int64, error) {
	potentialOldEntries, err := m.rClient.ZRangeByScore(
		ctx,
		getFlushAndDeleteQueueKey(),
		&redis.ZRangeBy{
			Min:    "0",
			Max:    strconv.FormatInt(cutoffTime.Unix(), 10),
			Offset: 0,
			Count:  1,
		},
	).Result()
	if err != nil {
		return sharedTypes.UUID{}, 0, 0, errors.Tag(
			err, "get old entries by score",
		)
	}
	if len(potentialOldEntries) == 0 {
		return sharedTypes.UUID{}, 0, 0, nil
	}
	// NOTE: The score of the returned member my not be above cutoffTime due to
	//        multiple pods racing and popping entries from the queue.
	//       This is OK as the score is mostly used for smoothing spikes only.
	entries, err := m.rClient.ZPopMin(
		ctx,
		getFlushAndDeleteQueueKey(),
		1,
	).Result()
	if err != nil {
		if err == redis.Nil {
			return sharedTypes.UUID{}, 0, 0, nil
		}
		return sharedTypes.UUID{}, 0, 0, err
	}
	if len(entries) == 0 {
		return sharedTypes.UUID{}, 0, 0, nil
	}
	raw, ok := entries[0].Member.(string)
	if !ok {
		return sharedTypes.UUID{}, 0, 0, errors.New("unexpected queue entry")
	}
	id, err := sharedTypes.ParseUUID(raw)
	if err != nil {
		return sharedTypes.UUID{}, 0, 0, errors.Tag(err, "unexpected queue entry")
	}
	return id, int64(entries[0].Score), 0, nil
}
