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

package editorEvents

import (
	"encoding/json"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

func (r *room) handleUpdate(msg sharedTypes.EditorEvent) error {
	var update sharedTypes.DocumentUpdate
	if err := json.Unmarshal(msg.Payload, &update); err != nil {
		return errors.Tag(err, "parse document update")
	}
	if err := update.Validate(); err != nil {
		return errors.Tag(err, "validate document update")
	}

	latency := sharedTypes.Timed{}
	if !update.Meta.IngestionTime.IsZero() {
		latency.SetBegin(update.Meta.IngestionTime)
		latency.End()
	}
	var bulkMessage types.WriteQueueEntry
	clients := r.Clients()
	for i, client := range clients.All {
		if clients.Removed.Has(i) {
			continue
		}
		if client.PublicId == update.Meta.Source {
			r.sendAckToSender(client, update, latency, msg.ProcessedBy)
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
		if !client.HasJoinedDoc(update.DocId) {
			continue
		}
		if bulkMessage.Msg == nil {
			blob, err := json.Marshal(sharedTypes.AppliedDocumentUpdate{
				DocId: update.DocId,
				Meta: sharedTypes.AppliedDocumentUpdateMeta{
					Source: update.Meta.Source,
					Type:   update.Meta.Type,
				},
				Op:      update.Op,
				Version: update.Version,
			})
			if err != nil {
				return errors.Tag(err, "serialize otUpdateApplied body")
			}
			resp := types.RPCResponse{
				Name:        "otUpdateApplied",
				Body:        blob,
				Latency:     latency,
				ProcessedBy: msg.ProcessedBy,
			}
			if bulkMessage, err = types.PrepareBulkMessage(&resp); err != nil {
				return err
			}
		}
		client.EnsureQueueMessage(bulkMessage)
	}
	return nil
}

func (r *room) sendAckToSender(client *types.Client, msg sharedTypes.DocumentUpdate, latency sharedTypes.Timed, processedBy string) {
	minUpdate := sharedTypes.DocumentUpdateAck{
		DocId:   msg.DocId,
		Version: msg.Version,
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
		ProcessedBy: processedBy,
	}
	client.EnsureQueueResponse(&resp)
}
