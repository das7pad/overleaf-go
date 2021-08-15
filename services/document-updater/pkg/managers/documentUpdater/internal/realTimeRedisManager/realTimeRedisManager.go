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

package realTimeRedisManager

import (
	"context"
	"encoding/json"

	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/services/document-updater/pkg/errors"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
)

type Manager interface {
	ConfirmUpdates(ctx context.Context, processed []types.DocumentUpdate) error

	GetPendingUpdatesForDoc(
		ctx context.Context,
		docId primitive.ObjectID,
	) ([]types.DocumentUpdate, error)

	GetUpdatesLength(
		ctx context.Context,
		docId primitive.ObjectID,
	) (int64, error)

	ReportError(
		ctx context.Context,
		docId primitive.ObjectID,
		err error,
	) error
}

func New(client redis.UniversalClient) Manager {
	return &manager{client: client}
}

type manager struct {
	client redis.UniversalClient
}

func getPendingUpdatesKey(docId primitive.ObjectID) string {
	return "PendingUpdates:{" + docId.Hex() + "}"
}

const maxOpsPerIteration = 10

func (m *manager) GetPendingUpdatesForDoc(ctx context.Context, docId primitive.ObjectID) ([]types.DocumentUpdate, error) {
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
	updates := make([]types.DocumentUpdate, len(raw))
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

func (m *manager) GetUpdatesLength(ctx context.Context, docId primitive.ObjectID) (int64, error) {
	n, err := m.client.LLen(ctx, getPendingUpdatesKey(docId)).Result()
	if err != nil {
		return 0, errors.Tag(err, "cannot get updates queue depth")
	}
	return n, nil
}

func (m *manager) ConfirmUpdates(ctx context.Context, processed []types.DocumentUpdate) error {
	_, err := m.client.Pipelined(ctx, func(p redis.Pipeliner) error {
		for _, update := range processed {
			if update.Dup {
				// minimal response
				update = types.DocumentUpdate{
					DocId: update.DocId,
					Dup:   true,
					Meta: types.DocumentUpdateMeta{
						Source: update.Meta.Source,
					},
					Version: update.Version,
				}
			}

			_, err := m.sendMessageVia(ctx, p, &types.AppliedOpsMessage{
				DocId:  update.DocId,
				Update: &update,
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

func (m *manager) ReportError(ctx context.Context, docId primitive.ObjectID, err error) error {
	message := &types.AppliedOpsMessage{
		DocId: docId,
	}
	if publicErr, ok := err.(errors.PublicError); ok {
		message.Error = publicErr.Public()
	} else {
		message.Error = &errors.JavaScriptError{
			Message: "hidden error in document-updater",
			Code:    "hidden",
		}
	}
	return m.sendMessage(ctx, message)
}

func (m *manager) sendMessage(ctx context.Context, message *types.AppliedOpsMessage) error {
	cmd, err := m.sendMessageVia(ctx, m.client, message)
	if err != nil {
		return err
	}
	if err = cmd.Err(); err != nil {
		return errors.Tag(err, "cannot send message")
	}
	return nil
}

func (m *manager) sendMessageVia(ctx context.Context, runner redis.Cmdable, message *types.AppliedOpsMessage) (*redis.IntCmd, error) {
	blob, err := json.Marshal(message)
	if err != nil {
		return nil, errors.Tag(err, "cannot serialize message")
	}
	channel := "applied-ops:" + message.DocId.Hex()
	return runner.Publish(ctx, channel, blob), nil
}
