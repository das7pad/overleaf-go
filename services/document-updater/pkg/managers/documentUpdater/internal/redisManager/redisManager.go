// Golang port of the Overleaf document-updater service
// Copyright (C) 2021 Jakob Ackermann <das7pad@outlook.com>
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

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/errors"

	"github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
)

type Manager interface {
	PutDocInMemory(
		ctx context.Context,
		projectId primitive.ObjectID,
		docId primitive.ObjectID,
		doc *types.Doc,
	) error

	RemoveDocFromMemory(
		ctx context.Context,
		projectId primitive.ObjectID,
		docId primitive.ObjectID,
	) error

	CheckOrSetProjectState(
		ctx context.Context,
		projectId primitive.ObjectID,
		newState string,
	) error

	ClearProjectState(
		ctx context.Context,
		projectId primitive.ObjectID,
	) error

	GetDoc(
		ctx context.Context,
		projectId primitive.ObjectID,
		docId primitive.ObjectID,
	) (*types.Doc, error)

	GetDocVersion(
		ctx context.Context,
		docId primitive.ObjectID,
	) (types.Version, error)

	GetPreviousDocUpdates(
		ctx context.Context,
		docId primitive.ObjectID,
		start types.Version,
		end types.Version,
	) ([]types.DocumentUpdate, error)

	GetPreviousDocUpdatesUnderLock(
		ctx context.Context,
		docId primitive.ObjectID,
		begin types.Version,
		end types.Version,
		docVersion types.Version,
	) ([]types.DocumentUpdate, error)

	UpdateDocument(
		ctx context.Context,
		docId primitive.ObjectID,
		doc *types.Doc,
		appliedUpdates []types.DocumentUpdate,
	) (int64, error)

	RenameDoc(
		ctx context.Context,
		projectId primitive.ObjectID,
		docId primitive.ObjectID,
		doc *types.Doc,
		update *types.RenameDocUpdate,
	) error

	ClearUnFlushedTime(
		ctx context.Context,
		docId primitive.ObjectID,
	) error

	GetDocIdsInProject(
		ctx context.Context,
		projectId primitive.ObjectID,
	) ([]primitive.ObjectID, error)

	GetDocTimestamps(
		ctx context.Context,
		docIds []primitive.ObjectID,
	) ([]int64, error)

	QueueFlushAndDeleteProject(
		ctx context.Context,
		projectId primitive.ObjectID,
	) error

	GetNextProjectToFlushAndDelete(
		ctx context.Context,
		cutoffTime int64,
	) (primitive.ObjectID, int64, int64, error)
}

func New(rClient redis.UniversalClient) Manager {
	return &manager{rClient: rClient}
}

const (
	DocOpsTTL       = 60 * time.Minute
	DocOpsMaxLength = 100
)

var (
	updateRangeNotAvailable = &errors.UpdateRangeNotAvailableError{}
)

type manager struct {
	rClient redis.UniversalClient
}

func getDocsInProjectKey(projectId primitive.ObjectID) string {
	return "DocsIn:{" + projectId.Hex() + "}"
}
func getProjectStateKey(projectId primitive.ObjectID) string {
	return "ProjectState:{" + projectId.Hex() + "}"
}
func getDocCoreKey(docId primitive.ObjectID) string {
	return "docCore:{" + docId.Hex() + "}"
}
func getDocVersionKey(docId primitive.ObjectID) string {
	return "DocVersion:{" + docId.Hex() + "}"
}
func getUnFlushedTimeKey(docId primitive.ObjectID) string {
	return "UnflushedTime:{" + docId.Hex() + "}"
}
func getLastUpdatedCtxKey(docId primitive.ObjectID) string {
	return "lastUpdatedCtx:{" + docId.Hex() + "}"
}
func getDocUpdatesKey(docId primitive.ObjectID) string {
	return "DocOps:{" + docId.Hex() + "}"
}
func getUncompressedHistoryOpsKey(docId primitive.ObjectID) string {
	return "UncompressedHistoryOps:{" + docId.Hex() + "}"
}
func getFlushAndDeleteQueueKey() string {
	return "DocUpdaterFlushAndDeleteQueue"
}

func (m *manager) PutDocInMemory(ctx context.Context, projectId primitive.ObjectID, docId primitive.ObjectID, doc *types.Doc) error {
	err := m.rClient.SAdd(ctx, getDocsInProjectKey(projectId), docId.Hex()).Err()
	if err != nil {
		return errors.Tag(err, "cannot record doc in project")
	}
	coreBlob, err := doc.DocCore.DoMarshalJSON()
	if err != nil {
		return errors.Tag(err, "cannot serialize DocCore")
	}
	vars := map[string]interface{}{
		getDocCoreKey(docId):    coreBlob,
		getDocVersionKey(docId): doc.Version.String(),
	}
	if err = m.rClient.MSet(ctx, vars).Err(); err != nil {
		return errors.Tag(err, "cannot persist in redis")
	}
	return nil
}

func (m *manager) RemoveDocFromMemory(ctx context.Context, projectId primitive.ObjectID, docId primitive.ObjectID) error {
	err := m.rClient.Del(
		ctx,
		getDocCoreKey(docId),
		getDocVersionKey(docId),
		getUnFlushedTimeKey(docId),
		getLastUpdatedCtxKey(docId),
	).Err()
	if err != nil {
		return errors.Tag(err, "cannot cleanup doc details")
	}

	_, err = m.rClient.Pipelined(ctx, func(p redis.Pipeliner) error {
		p.SRem(ctx, getDocsInProjectKey(projectId), docId.Hex())
		p.Del(ctx, getProjectStateKey(projectId))
		return nil
	})
	if err != nil {
		return errors.Tag(err, "cannot cleanup project tracking")
	}
	return nil
}

func (m *manager) CheckOrSetProjectState(ctx context.Context, projectId primitive.ObjectID, newState string) error {
	var res *redis.StringCmd
	_, err := m.rClient.TxPipelined(ctx, func(p redis.Pipeliner) error {
		res = p.GetSet(ctx, getProjectStateKey(projectId), newState)
		p.Expire(ctx, getProjectStateKey(projectId), 30*time.Minute)
		return nil
	})
	if err != nil && err != redis.Nil {
		return errors.Tag(err, "cannot check/swap state")
	}
	if res == nil || res.Val() != newState {
		return &errors.InvalidStateError{}
	}
	return nil
}

func (m *manager) ClearProjectState(ctx context.Context, projectId primitive.ObjectID) error {
	return m.rClient.Del(ctx, getProjectStateKey(projectId)).Err()
}

func (m *manager) GetDoc(ctx context.Context, projectId primitive.ObjectID, docId primitive.ObjectID) (*types.Doc, error) {
	res := m.rClient.MGet(
		ctx,
		getDocCoreKey(docId),
		getDocVersionKey(docId),
		getUnFlushedTimeKey(docId),
		getLastUpdatedCtxKey(docId),
	)
	if err := res.Err(); err != nil {
		return nil, errors.Tag(err, "cannot get doc details from redis")
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
	doc := &types.Doc{
		DocId: docId,
	}
	if err := doc.DocCore.DoUnmarshalJSON(blobs[0]); err != nil {
		return nil, errors.Tag(err, "cannot parse doc core")
	}
	if doc.ProjectId != projectId {
		return nil, &errors.NotAuthorizedError{}
	}

	if err := json.Unmarshal(blobs[1], &doc.Version); err != nil {
		return nil, errors.Tag(err, "cannot parse doc version")
	}
	if len(blobs[2]) != 0 {
		if err := json.Unmarshal(blobs[2], &doc.UnFlushedTime); err != nil {
			return nil, errors.Tag(err, "cannot parse doc un-flushed time")
		}
	}
	if len(blobs[3]) > 2 {
		if err := json.Unmarshal(blobs[3], &doc.LastUpdatedCtx); err != nil {
			return nil, errors.Tag(err, "cannot parse last updated ctx")
		}
	}
	return doc, nil
}

func (m *manager) GetDocVersion(ctx context.Context, docId primitive.ObjectID) (types.Version, error) {
	raw, err := m.rClient.Get(ctx, getDocVersionKey(docId)).Result()
	if err != nil {
		if err == redis.Nil {
			err = &errors.NotFoundError{}
		}
		return 0, errors.Tag(err, "cannot get version from redis")
	}
	var v types.Version
	if err = json.Unmarshal([]byte(raw), &v); err != nil {
		return 0, errors.Tag(err, "cannot parse version")
	}
	return v, nil
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

func (m *manager) GetPreviousDocUpdates(ctx context.Context, docId primitive.ObjectID, start types.Version, end types.Version) ([]types.DocumentUpdate, error) {
	if start == end {
		return make([]types.DocumentUpdate, 0), nil
	}
	keys := []string{
		getDocUpdatesKey(docId),
		getDocVersionKey(docId),
	}
	argv := []interface{}{
		start.String(),
		end.String(),
	}
	res, err := scriptGetPreviousDocUpdates.Run(ctx, m.rClient, keys, argv).Result()
	if err != nil {
		if strings.Contains(err.Error(), "overleaf:") {
			return nil, updateRangeNotAvailable
		}
		return nil, errors.Tag(err, "cannot get previous updates from redis")
	}
	stage0, isArr := res.([]interface{})
	if !isArr {
		err = errors.New("got non array")
		return nil, errors.Tag(err, "unexpected updates response from redis")
	}
	blobs := make([]string, end-start)
	for i, item := range stage0 {
		if s, isString := item.(string); isString {
			blobs[i] = s
			continue
		}
		update := (start + types.Version(i)).String()
		err = errors.New(
			"update not received as string: " + update,
		)
		return nil, errors.Tag(
			err, "unexpected updates response from redis",
		)
	}
	return m.parseDocumentUpdates(start, blobs)
}

func (m *manager) GetPreviousDocUpdatesUnderLock(ctx context.Context, docId primitive.ObjectID, begin types.Version, end types.Version, docVersion types.Version) ([]types.DocumentUpdate, error) {
	if begin == end {
		return nil, nil
	}
	n := int64(end - begin)
	offset := int64(docVersion - end)
	start := -n - offset
	stop := -1 - offset
	raw, err :=
		m.rClient.LRange(ctx, getDocUpdatesKey(docId), start, stop).Result()
	if err != nil {
		return nil, errors.Tag(err, "cannot get previous updates from redis")
	}
	if len(raw) != int(n) {
		return nil, updateRangeNotAvailable
	}
	return m.parseDocumentUpdates(begin, raw)
}

func (m *manager) parseDocumentUpdates(start types.Version, raw []string) ([]types.DocumentUpdate, error) {
	updates := make([]types.DocumentUpdate, len(raw))
	for i, s := range raw {
		update := types.DocumentUpdate{}
		if err := json.Unmarshal([]byte(s), &update); err != nil {
			return nil, errors.Tag(err, "cannot parse update")
		}
		if i == 0 && start != update.Version {
			return nil, updateRangeNotAvailable
		}
		updates[i] = update
	}
	return updates, nil
}

func (m *manager) UpdateDocument(ctx context.Context, docId primitive.ObjectID, doc *types.Doc, appliedUpdates []types.DocumentUpdate) (int64, error) {
	currentVersion, err := m.GetDocVersion(ctx, docId)
	if err != nil {
		return 0, errors.Tag(err, "cannot get doc version for validation")
	}
	nUpdatesOffset := types.Version(len(appliedUpdates))
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
		return 0, errors.Tag(err, "cannot serialize doc core")
	}
	appliedUpdatesBlobs := make([]interface{}, len(appliedUpdates))
	for i, update := range appliedUpdates {
		appliedUpdateBlob, err2 := json.Marshal(update)
		if err2 != nil {
			return 0, errors.Tag(err2, "cannot serialize applied update")
		}
		appliedUpdatesBlobs[i] = appliedUpdateBlob
	}
	lastUpdatedCtxBlob, err := json.Marshal(doc.LastUpdatedCtx)
	if err != nil {
		return 0, errors.Tag(err, "cannot serialize last updated ctx")
	}
	var uncompressedHistoryOpsRes *redis.IntCmd
	_, err = m.rClient.TxPipelined(ctx, func(p redis.Pipeliner) error {
		p.MSet(ctx, map[string]interface{}{
			getDocCoreKey(docId):        coreBlob,
			getDocVersionKey(docId):     doc.Version.String(),
			getLastUpdatedCtxKey(docId): lastUpdatedCtxBlob,
		})
		p.LTrim(ctx, getDocUpdatesKey(docId), -DocOpsMaxLength, -1)
		if len(appliedUpdatesBlobs) > 0 {
			p.RPush(ctx, getDocUpdatesKey(docId), appliedUpdatesBlobs...)
			p.Expire(ctx, getDocUpdatesKey(docId), DocOpsTTL)

			uncompressedHistoryOpsRes = p.RPush(
				ctx,
				getUncompressedHistoryOpsKey(docId),
				appliedUpdatesBlobs...,
			)
		}
		// NOTE: Node.JS is doing this in above branch.
		//       This might be a problem for ranges-only updates.
		now := time.Now().Unix()
		doc.UnFlushedTime = types.UnFlushedTime(now)
		p.SetNX(
			ctx,
			getUnFlushedTimeKey(docId),
			now,
			0,
		)

		return nil
	})
	if err != nil {
		return 0, errors.Tag(err, "cannot update doc in redis")
	}
	if uncompressedHistoryOpsRes != nil {
		return uncompressedHistoryOpsRes.Val(), nil
	}
	return -1, nil
}

func (m *manager) RenameDoc(ctx context.Context, projectId primitive.ObjectID, docId primitive.ObjectID, doc *types.Doc, update *types.RenameDocUpdate) error {
	doc.PathName = update.NewPathName
	if err := m.PutDocInMemory(ctx, projectId, docId, doc); err != nil {
		return errors.Tag(err, "cannot rewrite doc in redis")
	}
	return nil
}

func (m *manager) ClearUnFlushedTime(ctx context.Context, docId primitive.ObjectID) error {
	return m.rClient.Del(ctx, getUnFlushedTimeKey(docId)).Err()
}

func (m *manager) GetDocIdsInProject(ctx context.Context, projectId primitive.ObjectID) ([]primitive.ObjectID, error) {
	res := m.rClient.SMembers(ctx, getDocsInProjectKey(projectId))
	if err := res.Err(); err != nil {
		return nil, errors.Tag(err, "cannot get docs from redis")
	}
	rawIds := res.Val()
	docIds := make([]primitive.ObjectID, len(rawIds))
	for i, raw := range rawIds {
		id, err := primitive.ObjectIDFromHex(raw)
		if err != nil {
			return nil, errors.Tag(err, "cannot parse raw docId: "+raw)
		}
		docIds[i] = id
	}
	return docIds, nil
}

func (m *manager) GetDocTimestamps(ctx context.Context, docIds []primitive.ObjectID) ([]int64, error) {
	if len(docIds) == 0 {
		return nil, nil
	}
	commands := make([]*redis.StringCmd, len(docIds))
	// Note: The docs may be hosted on multiple shards. Pipelined is per shard.
	_, err := m.rClient.Pipelined(ctx, func(p redis.Pipeliner) error {
		for idx, id := range docIds {
			commands[idx] = p.Get(ctx, getLastUpdatedCtxKey(id))
		}
		return nil
	})
	if err != nil {
		return nil, errors.Tag(err, "cannot get timestamp from redis")
	}
	timestamps := make([]int64, len(commands))
	for i, cmd := range commands {
		raw := cmd.Val()
		if raw == "" {
			timestamps[i] = 0
		} else {
			var lastUpdatedCtx types.LastUpdatedCtx
			err2 := json.Unmarshal([]byte(raw), &lastUpdatedCtx)
			if err2 != nil {
				timestamps[i] = 0
			} else {
				timestamps[i] = lastUpdatedCtx.At
			}
		}
	}
	return timestamps, nil
}

const SmoothingOffset = int64(time.Second)

func (m *manager) QueueFlushAndDeleteProject(ctx context.Context, projectId primitive.ObjectID) error {
	smoothingOffset := time.Duration(rand.Int63n(SmoothingOffset))
	score := time.Now().Add(smoothingOffset).Unix()
	queueEntry := &redis.Z{
		Score:  float64(score),
		Member: projectId.Hex(),
	}
	return m.rClient.ZAdd(ctx, getFlushAndDeleteQueueKey(), queueEntry).Err()
}

func (m *manager) GetNextProjectToFlushAndDelete(ctx context.Context, cutoffTime int64) (primitive.ObjectID, int64, int64, error) {
	potentialOldEntries, err := m.rClient.ZRangeByScore(
		ctx,
		getFlushAndDeleteQueueKey(),
		&redis.ZRangeBy{
			Min:    "0",
			Max:    strconv.FormatInt(cutoffTime, 10),
			Offset: 0,
			Count:  1,
		},
	).Result()
	if err != nil {
		return primitive.NilObjectID, 0, 0, errors.Tag(
			err, "cannot get old entries by score",
		)
	}
	if len(potentialOldEntries) == 0 {
		return primitive.NilObjectID, 0, 0, nil
	}
	// NOTE: The score of the returned member my not be above cutoffTime due to
	//        multiple pods racing and popping entries from the queue.
	//       This is OK as the score is mostly used for smoothing spikes only.
	entries, err := m.rClient.ZPopMin(
		ctx,
		getFlushAndDeleteQueueKey(),
		1,
	).Result()
	if len(entries) == 0 {
		return primitive.NilObjectID, 0, 0, nil
	}
	var raw string
	switch val := entries[0].Member.(type) {
	case string:
		raw = val
	default:
		return primitive.NilObjectID, 0, 0, errors.New("unexpected queue entry")
	}
	id, err := primitive.ObjectIDFromHex(raw)
	if err != nil {
		return primitive.NilObjectID, 0, 0, errors.Tag(err, "unexpected queue entry")
	}
	return id, int64(entries[0].Score), 0, nil
}