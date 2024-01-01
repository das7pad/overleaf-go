// Golang port of Overleaf
// Copyright (C) 2022-2024 Jakob Ackermann <das7pad@outlook.com>
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

	"github.com/go-redis/redis/v8"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

const (
	batchSizeProcessUpdates = 100
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
		return errors.Tag(err, "record doc has history")
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
			ids := projectId.Concat('/', docId)
			err = errors.Tag(err, "flush history for "+ids)
			log.Println(err.Error())
		}
	}()
}

func getUncompressedHistoryOpsKey(docId sharedTypes.UUID) string {
	b := make([]byte, 0, 24+36+1)
	b = append(b, "UncompressedHistoryOps:{"...)
	b = docId.Append(b)
	b = append(b, '}')
	return string(b)
}

func (m *manager) FlushDoc(ctx context.Context, projectId, docId sharedTypes.UUID) error {
	queueKey := getUncompressedHistoryOpsKey(docId)
	projectTracking := getProjectTrackingKey(projectId)
	for {
		err := m.rl.RunWithLock(ctx, docId, func(ctx context.Context) error {
			rawUpdates, err := m.client.LRange(
				ctx,
				queueKey,
				0,
				batchSizeProcessUpdates,
			).Result()
			if err != nil {
				return errors.Tag(err, "get updates from redis")
			}
			updates := make([]sharedTypes.DocumentUpdate, len(rawUpdates))
			for i, update := range rawUpdates {
				err = json.Unmarshal([]byte(update), &updates[i])
				if err != nil {
					return errors.Tag(
						err, fmt.Sprintf("decode update %d", i),
					)
				}
			}

			err = m.persistUpdates(ctx, projectId, docId, updates)
			if err != nil {
				return errors.Tag(err, "persist updates")
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
				return errors.Tag(err, "pop from redis queue")
			}

			if d, _ := queueDepthCmd.Result(); d != 0 {
				return errPartialFlush
			}

			// The queue is empty. Bonus: cleanup the project tracking.
			_ = m.client.SRem(ctx, projectTracking, docId.String()).Err()

			return nil
		})
		if err == errPartialFlush {
			continue
		}
		if err != nil {
			return err
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

	// mergeComponents updates
	dh := mergeInserts(mergeUpdates(updates))

	// insert
	if err = m.dhm.InsertBulk(ctx, docId, dh); err != nil {
		return errors.Tag(err, "insert history into db")
	}
	return nil
}
