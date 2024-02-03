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

package realTimeRedisManager

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/pubSub/channel"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Manager interface {
	ConfirmUpdates(ctx context.Context, projectId sharedTypes.UUID, processed []sharedTypes.DocumentUpdate) error
	GetPendingUpdatesForDoc(ctx context.Context, docId sharedTypes.UUID) ([]sharedTypes.DocumentUpdate, error)
	GetUpdatesLength(ctx context.Context, docId sharedTypes.UUID) (int64, error)
	QueueUpdate(ctx context.Context, docId sharedTypes.UUID, update sharedTypes.DocumentUpdate) error
	ReportError(ctx context.Context, projectId, docId sharedTypes.UUID, err error) error
}

func New(client redis.UniversalClient) (Manager, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, errors.Tag(err, "get hostname")
	}

	return &manager{
		c:        channel.NewWriter(client, "editor-events"),
		client:   client,
		hostname: hostname,
	}, nil
}

type manager struct {
	c        channel.Writer
	client   redis.UniversalClient
	hostname string
}

func getPendingUpdatesKey(docId sharedTypes.UUID) string {
	b := make([]byte, 0, 16+36+1)
	b = append(b, "PendingUpdates:{"...)
	b = docId.Append(b)
	b = append(b, '}')
	return string(b)
}

const maxOpsPerIteration = 10

func (m *manager) GetPendingUpdatesForDoc(ctx context.Context, docId sharedTypes.UUID) ([]sharedTypes.DocumentUpdate, error) {
	var result *redis.StringSliceCmd
	_, err := m.client.TxPipelined(ctx, func(p redis.Pipeliner) error {
		key := getPendingUpdatesKey(docId)
		result = p.LRange(ctx, key, 0, maxOpsPerIteration-1)
		p.LTrim(ctx, key, maxOpsPerIteration, -1)
		return nil
	})
	if err != nil {
		return nil, errors.Tag(err, "fetch pending updates")
	}
	raw := result.Val()
	updates := make([]sharedTypes.DocumentUpdate, len(raw))
	for i, blob := range raw {
		err = json.Unmarshal([]byte(blob), &updates[i])
		if err != nil {
			return nil, errors.Tag(err, "deserialize update")
		}
		if err = updates[i].Validate(); err != nil {
			return nil, errors.Tag(err, "update invalid")
		}
	}
	return updates, nil
}

func (m *manager) GetUpdatesLength(ctx context.Context, docId sharedTypes.UUID) (int64, error) {
	n, err := m.client.LLen(ctx, getPendingUpdatesKey(docId)).Result()
	if err != nil {
		return 0, errors.Tag(err, "get updates queue depth")
	}
	return n, nil
}

func (m *manager) QueueUpdate(ctx context.Context, docId sharedTypes.UUID, update sharedTypes.DocumentUpdate) error {
	// Hard code document id
	update.DocId = docId
	// Dup is an output only field
	update.Dup = false
	// Ingestion time is tracked internally only
	update.Meta.IngestionTime = time.Now()

	if err := update.Validate(); err != nil {
		return err
	}

	blob, err := json.Marshal(update)
	if err != nil {
		return errors.Tag(err, "encode update")
	}

	err = m.client.RPush(ctx, getPendingUpdatesKey(docId), blob).Err()
	if err != nil {
		return errors.Tag(err, "queue update")
	}
	return nil
}

func (m *manager) ConfirmUpdates(ctx context.Context, projectId sharedTypes.UUID, processed []sharedTypes.DocumentUpdate) error {
	_, err := m.client.Pipelined(ctx, func(p redis.Pipeliner) error {
		for _, update := range processed {
			if update.Dup {
				// minimal response
				update = sharedTypes.DocumentUpdate{
					DocId: update.DocId,
					Dup:   true,
					Meta: sharedTypes.DocumentUpdateMeta{
						Source:        update.Meta.Source,
						IngestionTime: update.Meta.IngestionTime,
					},
					Version: update.Version,
				}
			}

			blob, err := json.Marshal(update)
			if err != nil {
				return err
			}
			_, err = m.c.PublishVia(ctx, p, &sharedTypes.EditorEvent{
				RoomId:      projectId,
				Message:     sharedTypes.OtUpdateApplied,
				Payload:     blob,
				ProcessedBy: m.hostname,
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

func (m *manager) ReportError(ctx context.Context, projectId, docId sharedTypes.UUID, err error) error {
	blob, err := json.Marshal(sharedTypes.AppliedOpsErrorMeta{
		DocId: docId,
		Error: errors.JavaScriptError{
			Message: errors.GetPublicMessage(
				err, "hidden error in document-updater",
			),
		},
	})
	if err != nil {
		return errors.Tag(err, "serialize error meta")
	}
	return m.c.Publish(ctx, &sharedTypes.EditorEvent{
		RoomId:      projectId,
		Message:     sharedTypes.OtUpdateError,
		Payload:     blob,
		ProcessedBy: m.hostname,
	})
}
