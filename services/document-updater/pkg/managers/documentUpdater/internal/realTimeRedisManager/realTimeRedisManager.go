// Golang port of Overleaf
// Copyright (C) 2021-2023 Jakob Ackermann <das7pad@outlook.com>
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

	"github.com/go-redis/redis/v8"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/pubSub/channel"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Manager interface {
	ConfirmUpdates(ctx context.Context, projectId sharedTypes.UUID, processed []sharedTypes.DocumentUpdate) error
	GetPendingUpdatesForDoc(ctx context.Context, docId sharedTypes.UUID) ([]sharedTypes.DocumentUpdate, error)
	GetUpdatesLength(ctx context.Context, docId sharedTypes.UUID) (int64, error)
	ReportError(ctx context.Context, projectId, docId sharedTypes.UUID, err error) error
}

func New(client redis.UniversalClient) (Manager, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, errors.Tag(err, "cannot get hostname")
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
	return "PendingUpdates:{" + docId.String() + "}"
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
		return nil, errors.Tag(err, "cannot fetch pending updates")
	}
	raw := result.Val()
	updates := make([]sharedTypes.DocumentUpdate, len(raw))
	for i, blob := range raw {
		err = json.Unmarshal([]byte(blob), &updates[i])
		if err != nil {
			return nil, errors.Tag(err, "cannot deserialize update")
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
		return 0, errors.Tag(err, "cannot get updates queue depth")
	}
	return n, nil
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
			_, err = m.c.PublishVia(ctx, p, &sharedTypes.EditorEventsMessage{
				RoomId:      projectId,
				Message:     "otUpdateApplied",
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
	blob, err := json.Marshal([]interface{}{
		errors.JavaScriptError{
			Message: errors.GetPublicMessage(
				err, "hidden error in document-updater",
			),
		},
		sharedTypes.AppliedOpsErrorMeta{DocId: docId},
	})
	if err != nil {
		return err
	}
	return m.c.Publish(ctx, &sharedTypes.EditorEventsMessage{
		RoomId:      projectId,
		Message:     "otUpdateError",
		Payload:     blob,
		ProcessedBy: m.hostname,
	})
}
