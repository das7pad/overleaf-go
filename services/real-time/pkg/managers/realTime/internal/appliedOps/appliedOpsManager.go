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
	"log"
	"math/rand"
	"strconv"

	"github.com/go-redis/redis/v8"

	"github.com/das7pad/real-time/pkg/errors"
	"github.com/das7pad/real-time/pkg/managers/realTime/internal/broadcaster"
	"github.com/das7pad/real-time/pkg/managers/realTime/internal/channel"
	"github.com/das7pad/real-time/pkg/types"
)

type Manager interface {
	broadcaster.Broadcaster
	QueueUpdate(rpc *types.RPC, update *types.DocumentUpdate) error
}

func New(ctx context.Context, options *types.Options, client redis.UniversalClient) (Manager, error) {
	c, err := channel.New(ctx, options, client, "applied-ops")
	if err != nil {
		return nil, err
	}
	b := broadcaster.New(ctx, c)
	m := manager{
		Broadcaster:                  b,
		channel:                      c,
		client:                       client,
		pendingUpdatesListShardCount: options.PendingUpdatesListShardCount,
	}
	go m.listen()
	return &m, nil
}

type manager struct {
	broadcaster.Broadcaster

	channel                      channel.Manager
	client                       redis.UniversalClient
	pendingUpdatesListShardCount int64
}

func (m *manager) listen() {
	for raw := range m.channel.Listen() {
		var msg types.AppliedOpsMessage
		if err := json.Unmarshal([]byte(raw), &msg); err != nil {
			log.Println("cannot parse appliedOps message: " + err.Error())
			continue
		}
		if err := m.handleMessage(&msg); err != nil {
			log.Println("cannot handle appliedOps message: " + err.Error())
			continue
		}
	}
}

func (m *manager) handleMessage(msg *types.AppliedOpsMessage) error {
	if err := msg.Validate(); err != nil {
		return err
	}

	if msg.Error != nil {
		return m.handleError(msg)
	} else {
		return m.handleUpdate(msg)
	}
}
func (m *manager) handleError(msg *types.AppliedOpsMessage) error {
	resp := types.RPCResponse{
		Error:      msg.Error,
		Name:       "otUpdateError",
		FatalError: true,
	}
	bulkMessage, err := types.PrepareBulkMessage(&resp)
	if err != nil {
		return err
	}
	for _, client := range m.GetClients(msg.DocId) {
		client.EnsureQueueMessage(bulkMessage)
	}
	return nil
}

func (m *manager) handleUpdate(msg *types.AppliedOpsMessage) error {
	if err := msg.Validate(); err != nil {
		return err
	}

	update, err := msg.Update()
	if err != nil {
		return err
	}
	isComment := update.Ops.HasCommentOp()
	source := update.Meta.Source
	resp := types.RPCResponse{
		Name: "otUpdateApplied",
		Body: msg.UpdateRaw,
	}
	bulkMessage, err := types.PrepareBulkMessage(&resp)
	if err != nil {
		return err
	}
	for _, client := range m.GetClients(msg.DocId) {
		if client.PublicId == source {
			m.sendAckToSender(client, update)
			if update.Dup {
				// Only send an ack to the sender, then stop.
				break
			}
			continue
		}
		if update.Dup {
			// Only send an ack to the sender.
			continue
		}
		if isComment && !client.HasCapability(types.CanSeeComments) {
			continue
		}
		client.EnsureQueueMessage(bulkMessage)
	}
	return nil
}

func (m *manager) sendAckToSender(client *types.Client, update *types.DocumentUpdate) {
	minUpdate := types.MinimalDocumentUpdate{
		DocId:   update.DocId,
		Version: update.Version,
	}
	body, err := json.Marshal(minUpdate)
	if err != nil {
		client.TriggerDisconnect()
		return
	}
	resp := types.RPCResponse{
		Body: body,
		Name: "otUpdateApplied",
	}
	client.EnsureQueueResponse(&resp)
}

func (m *manager) getPendingUpdatesListKey() string {
	shard := rand.Int63n(m.pendingUpdatesListShardCount)
	if shard == 0 {
		return "pending-updates-list"
	}
	return "pending-updates-list-" + strconv.FormatInt(shard, 10)
}

func (m *manager) QueueUpdate(rpc *types.RPC, update *types.DocumentUpdate) error {
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
