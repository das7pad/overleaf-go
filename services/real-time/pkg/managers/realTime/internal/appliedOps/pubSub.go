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

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/managers/realTime/internal/broadcaster"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
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
	var msg sharedTypes.AppliedOpsMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		log.Println("cannot parse appliedOps message: " + err.Error())
		return
	}
	if err := msg.Validate(); err != nil {
		log.Println("cannot validate appliedOps message: " + err.Error())
		return
	}
	if msg.HealthCheck {
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

func (r *DocRoom) handleError(msg *sharedTypes.AppliedOpsMessage) error {
	blob, err := json.Marshal(&sharedTypes.AppliedOpsMessage{
		DocId: msg.DocId,
	})
	if err != nil {
		return errors.Tag(err, "cannot compose minimal error message")
	}
	resp := types.RPCResponse{
		Error:       msg.Error,
		Name:        "otUpdateError",
		Body:        blob,
		ProcessedBy: msg.ProcessedBy,
		FatalError:  true,
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

func (r *DocRoom) handleUpdate(msg *sharedTypes.AppliedOpsMessage) error {
	update := msg.Update
	latency := sharedTypes.Timed{}
	if update.Meta.IngestionTime != nil {
		latency.SetBegin(*update.Meta.IngestionTime)
		latency.End()
		update.Meta.IngestionTime = nil
	}
	isComment := update.Op.HasComment()
	source := update.Meta.Source
	blob, err := json.Marshal(&update)
	resp := types.RPCResponse{
		Name:        "otUpdateApplied",
		Body:        blob,
		Latency:     latency,
		ProcessedBy: msg.ProcessedBy,
	}
	bulkMessage, err := types.PrepareBulkMessage(&resp)
	if err != nil {
		return err
	}
	for _, client := range r.Clients() {
		if client.PublicId == source {
			r.sendAckToSender(client, msg, latency)
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

func (r *DocRoom) sendAckToSender(client *types.Client, msg *sharedTypes.AppliedOpsMessage, latency sharedTypes.Timed) {
	minUpdate := sharedTypes.DocumentUpdateAck{
		DocId:   msg.Update.DocId,
		Version: msg.Update.Version,
	}
	body, err := json.Marshal(minUpdate)
	if err != nil {
		client.TriggerDisconnect()
		return
	}
	resp := types.RPCResponse{
		Body:        body,
		Name:        "otUpdateApplied",
		Latency:     latency,
		ProcessedBy: msg.ProcessedBy,
	}
	client.EnsureQueueResponse(&resp)
}
