// Golang port of Overleaf
// Copyright (C) 2022 Jakob Ackermann <das7pad@outlook.com>
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

package flush

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/docHistory"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/sharejs/types/text"
)

const (
	batchSizeProcessUpdates = 100
	maxMergeSize            = 2 * 1024 * 1024
)

var errPartialFlush = errors.New("partial flush")

func shouldFlush(nUpdates, queueDepth int64) bool {
	if nUpdates < 1 || queueDepth < 1 {
		return false
	}
	before := (queueDepth - nUpdates) / batchSizeProcessUpdates
	after := queueDepth / batchSizeProcessUpdates
	return before != after
}

func (m *manager) RecordAndFlushHistoryOps(ctx context.Context, projectId, docId sharedTypes.UUID, nUpdates, queueDepth int64) error {
	key := getProjectTrackingKey(projectId)
	err := m.client.SAdd(ctx, key, docId.String()).Err()
	if err != nil {
		return errors.Tag(err, "cannot record doc has history")
	}

	if shouldFlush(nUpdates, queueDepth) {
		m.FlushDocInBackground(projectId, docId)
	}
	return nil
}

func (m *manager) FlushDocInBackground(projectId, docId sharedTypes.UUID) {
	go func() {
		err := m.FlushDoc(context.Background(), projectId, docId)
		if err != nil {
			ids := projectId.String() + "/" + docId.String()
			err = errors.Tag(err, "cannot flush history for "+ids)
			log.Println(err.Error())
		}
	}()
}

func (m *manager) FlushDoc(ctx context.Context, projectId, docId sharedTypes.UUID) error {
	queueKey := "UncompressedHistoryOps:{" + docId.String() + "}"
	projectTracking := getProjectTrackingKey(projectId)
	for {
		var err error
		lockErr := m.rl.RunWithLock(ctx, docId, func(ctx context.Context) {
			rawUpdates, errGetUpdates := m.client.LRange(
				ctx,
				queueKey,
				0,
				batchSizeProcessUpdates,
			).Result()
			if errGetUpdates != nil {
				err = errors.Tag(errGetUpdates, "cannot get updates from redis")
				return
			}
			updates := make([]sharedTypes.DocumentUpdate, len(rawUpdates))
			for i, update := range rawUpdates {
				err = json.Unmarshal([]byte(update), &updates[i])
				if err != nil {
					err = errors.Tag(
						err,
						fmt.Sprintf("cannot decode update %d", i),
					)
					return
				}
			}

			err = m.persistUpdates(ctx, projectId, docId, updates)
			if err != nil {
				err = errors.Tag(err, "cannot persist updates")
				return
			}

			var queueDepthCmd *redis.IntCmd
			_, err = m.client.Pipelined(ctx, func(p redis.Pipeliner) error {
				for _, update := range rawUpdates {
					p.LRem(ctx, queueKey, 1, update)
				}
				queueDepthCmd = p.LLen(ctx, queueKey)
				return nil
			})
			if err != nil {
				err = errors.Tag(err, "cannot pop from redis queue")
				return
			}

			if d, _ := queueDepthCmd.Result(); d != 0 {
				err = errPartialFlush
				return
			}

			// The queue is empty. Bonus: cleanup the project tracking.
			_ = m.client.SRem(ctx, projectTracking, docId.String()).Err()
		})
		if err == errPartialFlush {
			continue
		}
		if err != nil {
			return err
		}
		if lockErr != nil {
			return lockErr
		}
		return nil
	}
}

func (m *manager) persistUpdates(ctx context.Context, projectId, docId sharedTypes.UUID, updates []sharedTypes.DocumentUpdate) error {
	if len(updates) == 0 {
		return nil
	}

	// validate update sequence
	for i := 1; i < len(updates); i++ {
		if updates[i].Version <= updates[i-1].Version {
			return &errors.InvalidStateError{Msg: "non linear versions"}
		}
	}

	// validate sync status between db and redis
	v, err := m.dhm.GetLastVersion(ctx, projectId, docId)
	if err != nil {
		return err
	}
	if v >= updates[0].Version {
		// The last batch faced an error when removing updates from redis after
		//  they were persisted to postgres. Do not process them again.
		for len(updates) > 0 && v >= updates[0].Version {
			updates = updates[1:]
		}
		if len(updates) == 0 {
			return nil
		}
	}
	if updates[0].Version != v+1 {
		log.Printf(
			"%s/%s: incomplete history: version jump %d -> %d",
			projectId, docId, v, updates[0].Version,
		)
	}

	// merge updates
	dh := mergeUpdates(mergeOps(updates))

	// insert
	if err = m.dhm.InsertBulk(ctx, docId, dh); err != nil {
		return errors.Tag(err, "cannot insert history into db")
	}
	return nil
}

func mergeOps(updates []sharedTypes.DocumentUpdate) []docHistory.ForInsert {
	totalComponents := 0
	for i := 0; i < len(updates); i++ {
		totalComponents += len(updates[i].Op)
	}

	dhSingle := make([]docHistory.ForInsert, 0, totalComponents)
	dhSingle = append(dhSingle, docHistory.ForInsert{
		UserId:  updates[0].Meta.UserId,
		Version: updates[0].Version,
		StartAt: updates[0].Meta.Timestamp.ToTime(),
		EndAt:   updates[0].Meta.Timestamp.ToTime(),
		Op: sharedTypes.Op{
			updates[0].Op[0],
		},
	})
	updates[0].Op = updates[0].Op[1:]
	for _, update := range updates {
		t := update.Meta.Timestamp.ToTime()
		for _, secondC := range update.Op {
			tail := &dhSingle[len(dhSingle)-1]
			firstC := tail.Op[0]
			switch {
			case tail.UserId != update.Meta.UserId ||
				t.Sub(tail.EndAt) > time.Minute:
				// we need to create a new element
			case firstC.IsInsertion() &&
				secondC.IsInsertion() &&
				firstC.Position <= secondC.Position &&
				secondC.Position <= firstC.Position+len(firstC.Insertion) &&
				len(firstC.Insertion)+len(secondC.Insertion) < maxMergeSize:
				// merge the two overlapping insertions
				tail.EndAt = t
				tail.Version = update.Version
				tail.Op[0].Insertion = text.InjectInPlace(
					firstC.Insertion,
					secondC.Position-firstC.Position,
					secondC.Insertion,
				)
				continue
			case firstC.IsDeletion() &&
				secondC.IsDeletion() &&
				firstC.Position <= secondC.Position &&
				firstC.Position <= secondC.Position+len(secondC.Deletion) &&
				len(firstC.Deletion)+len(secondC.Deletion) < maxMergeSize:
				// merge the two overlapping deletions
				tail.EndAt = t
				tail.Version = update.Version
				tail.Op[0].Deletion = text.InjectInPlace(
					secondC.Deletion,
					firstC.Position-secondC.Position,
					firstC.Deletion,
				)
				continue
			case firstC.Position == secondC.Position &&
				(firstC.IsInsertion() &&
					string(firstC.Insertion) == string(secondC.Deletion) ||
					firstC.IsDeletion() &&
						string(firstC.Deletion) == string(secondC.Insertion)):
				// noop: insert + delete or delete + insert of the same text
				tail.EndAt = t
				tail.Version = update.Version
				tail.Op[0] = sharedTypes.Component{Position: firstC.Position}
				continue
			case firstC.IsInsertion() &&
				secondC.IsDeletion() &&
				firstC.Position <= secondC.Position &&
				secondC.Position+len(secondC.Deletion) <=
					firstC.Position+len(firstC.Insertion):
				// merge the insert followed by a deletion
				offset := secondC.Position - firstC.Position
				tail.EndAt = t
				tail.Version = update.Version
				nDeleted := len(secondC.Deletion)
				s := firstC.Insertion[:len(firstC.Insertion)-nDeleted]
				copy(s[offset:], firstC.Insertion[offset+nDeleted:])
				tail.Op[0].Insertion = s
				continue
			case firstC.IsDeletion() &&
				secondC.IsInsertion() &&
				firstC.Position == secondC.Position:
				// merge the (partial) overlap
				diff := text.Diff(firstC.Deletion, secondC.Insertion)
				dhSingle = dhSingle[:len(dhSingle)-1]
				for _, component := range diff {
					component.Position += firstC.Position
					dhSingle = append(dhSingle, docHistory.ForInsert{
						UserId:  update.Meta.UserId,
						Version: update.Version,
						StartAt: tail.StartAt,
						EndAt:   t,
						Op:      sharedTypes.Op{component},
					})
				}
				continue
			}
			// fallback: keep the two separated
			dhSingle = append(dhSingle, docHistory.ForInsert{
				UserId:  update.Meta.UserId,
				Version: update.Version,
				StartAt: t,
				EndAt:   t,
				Op:      sharedTypes.Op{secondC},
			})
		}
	}
	return dhSingle
}

func mergeUpdates(dhSingle []docHistory.ForInsert) []docHistory.ForInsert {
	dh := make([]docHistory.ForInsert, 0, len(dhSingle))
	var maxVersion sharedTypes.Version
	for _, update := range dhSingle {
		maxVersion = update.Version
		if update.Op[0].IsNoOp() {
			continue
		}
		if len(dh) == 0 {
			dh = append(dh, update)
			continue
		}
		tail := &dh[len(dh)-1]
		if update.UserId == tail.UserId &&
			(update.Version == tail.Version ||
				update.StartAt.Sub(tail.EndAt) <= time.Minute) {
			tail.EndAt = update.EndAt
			tail.Version = update.Version
			tail.Op = append(tail.Op, update.Op[0])
		} else {
			dh = append(dh, update)
		}
	}

	// Ensure that we have at least one entry for transporting the maxVersion.
	// We need to persist the maxVersion for detecting history jumps.
	if len(dh) == 0 {
		for _, update := range dhSingle {
			if update.Op[0].IsNoOp() {
				dh = dh[:1]
				dh[0] = update
			}
		}
	}
	dh[len(dh)-1].Version = maxVersion
	return dh
}
