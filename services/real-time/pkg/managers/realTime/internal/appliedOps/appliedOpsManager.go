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

package appliedOps

import (
	"context"
	"encoding/json"

	"github.com/go-redis/redis/v8"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/pubSub/channel"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater"
	documentUpdaterTypes "github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/managers/realTime/internal/broadcaster"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

type Manager interface {
	broadcaster.Broadcaster
	QueueUpdate(ctx context.Context, rpc *types.RPC, update *sharedTypes.DocumentUpdate) error
}

func New(client redis.UniversalClient, dum documentUpdater.Manager) Manager {
	c := channel.New(client, "applied-ops")
	b := broadcaster.New(c, newRoom)
	return &manager{
		Broadcaster:              b,
		client:                   client,
		getPendingUpdatesListKey: dum.GetPendingUpdatesListKey,
	}
}

type manager struct {
	broadcaster.Broadcaster

	client                   redis.UniversalClient
	getPendingUpdatesListKey func() documentUpdaterTypes.PendingUpdatesListKey
}

func (m *manager) QueueUpdate(ctx context.Context, rpc *types.RPC, update *sharedTypes.DocumentUpdate) error {
	blob, err := json.Marshal(update)
	if err != nil {
		return errors.Tag(err, "cannot encode update")
	}

	docId := rpc.Client.DocId
	pendingUpdateKey := "PendingUpdates:{" + docId.String() + "}"
	if err = m.client.RPush(ctx, pendingUpdateKey, blob).Err(); err != nil {
		return errors.Tag(err, "cannot queue update")
	}

	shardKey := m.getPendingUpdatesListKey().String()
	docKey := rpc.Client.ProjectId.String() + ":" + docId.String()
	if err = m.client.RPush(ctx, shardKey, docKey).Err(); err != nil {
		return errors.Tag(
			err,
			"cannot notify shard about new queue entry",
		)
	}
	return nil
}
