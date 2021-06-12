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
	"encoding/json"
	"log"

	"github.com/das7pad/real-time/pkg/managers/realTime/internal/broadcaster"
	"github.com/das7pad/real-time/pkg/types"
)

type DocRoom struct {
	*broadcaster.TrackingRoom
}

func newRoom(room *broadcaster.TrackingRoom) broadcaster.Room {
	return &DocRoom{
		TrackingRoom: room,
	}
}

func (r *DocRoom) Handle(raw string) {
	var msg types.AppliedOpsMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		log.Println("cannot parse appliedOps message: " + err.Error())
		return
	}
	if err := msg.Validate(); err != nil {
		log.Println("cannot validate appliedOps message: " + err.Error())
		return
	}
	var err error
	if msg.Error != nil {
		err = r.handleError(&msg)
	} else {
		err = r.handleUpdate(&msg)
	}
	if err != nil {
		log.Println("cannot handle appliedOps message: " + err.Error())
		return
	}
}

func (r *DocRoom) handleError(msg *types.AppliedOpsMessage) error {
	resp := types.RPCResponse{
		Error:      msg.Error,
		Name:       "otUpdateError",
		FatalError: true,
	}
	bulkMessage, err := types.PrepareBulkMessage(&resp)
	if err != nil {
		return err
	}
	for _, client := range r.Clients() {
		client.EnsureQueueMessage(bulkMessage)
	}
	return nil
}

func (r *DocRoom) handleUpdate(msg *types.AppliedOpsMessage) error {
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
	for _, client := range r.Clients() {
		if client.PublicId == source {
			r.sendAckToSender(client, update)
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

func (r *DocRoom) sendAckToSender(client *types.Client, update *types.DocumentUpdate) {
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
