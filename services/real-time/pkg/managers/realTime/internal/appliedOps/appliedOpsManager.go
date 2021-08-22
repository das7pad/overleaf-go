// Golang port of the Overleaf real-time service
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

package appliedOps

import (
	"context"
	"encoding/json"
	"math/rand"
	"strconv"

	"github.com/go-redis/redis/v8"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/managers/realTime/internal/broadcaster"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/managers/realTime/internal/channel"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

type Manager interface {
	broadcaster.Broadcaster
	QueueUpdate(rpc *types.RPC, update *sharedTypes.DocumentUpdate) error
}

func New(ctx context.Context, options *types.Options, client redis.UniversalClient) (Manager, error) {
	c, err := channel.New(ctx, client, "applied-ops")
	if err != nil {
		return nil, err
	}
	b := broadcaster.New(ctx, c, newRoom)
	m := manager{
		Broadcaster:                  b,
		channel:                      c,
		client:                       client,
		pendingUpdatesListShardCount: options.PendingUpdatesListShardCount,
	}
	return &m, nil
}

type manager struct {
	broadcaster.Broadcaster

	channel                      channel.Manager
	client                       redis.UniversalClient
	pendingUpdatesListShardCount int64
}

func (m *manager) getPendingUpdatesListKey() string {
	shard := rand.Int63n(m.pendingUpdatesListShardCount)
	if shard == 0 {
		return "pending-updates-list"
	}
	return "pending-updates-list-" + strconv.FormatInt(shard, 10)
}

func (m *manager) QueueUpdate(rpc *types.RPC, update *sharedTypes.DocumentUpdate) error {
	blob, err := json.Marshal(update)
	if err != nil {
		return errors.Tag(err, "cannot encode update")
	}

	docId := rpc.Client.DocId
	pendingUpdateKey := "PendingUpdates:{" + docId.Hex() + "}"
	if err = m.client.RPush(rpc, pendingUpdateKey, blob).Err(); err != nil {
		return errors.Tag(err, "cannot queue update")
	}

	shardKey := m.getPendingUpdatesListKey()
	docKey := rpc.Client.ProjectId.Hex() + ":" + docId.Hex()
	if err = m.client.RPush(rpc, shardKey, docKey).Err(); err != nil {
		return errors.Tag(
			err,
			"cannot notify shard about new queue entry",
		)
	}
	return nil
}
