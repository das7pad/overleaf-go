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

package editorEvents

import (
	"encoding/json"
	"log"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

func (r *room) Handle(raw string) {
	var msg sharedTypes.EditorEventsMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		log.Println("parse editorEvents message: " + err.Error())
		return
	}
	if err := msg.Validate(); err != nil {
		log.Println("validate editorEvents message: " + err.Error())
		return
	}
	var err error
	switch msg.Message {
	case "otUpdateApplied":
		err = r.handleUpdate(msg)
	default:
		err = r.handleMessage(msg)
	}
	if err != nil {
		log.Println("handle editorEvents message: " + err.Error())
		return
	}
}

func (r *room) handleMessage(msg sharedTypes.EditorEventsMessage) error {
	resp := types.RPCResponse{
		Name:        msg.Message,
		Body:        msg.Payload,
		ProcessedBy: msg.ProcessedBy,
	}
	var capability types.CapabilityComponent
	var bulkMessage types.WriteQueueEntry
	for _, client := range r.Clients() {
		if client.PublicId == msg.Source {
			continue
		}
		if capability == 0 {
			capability = getRequiredCapabilityForMessage(msg.Message)
		}
		if !client.HasCapability(capability) {
			continue
		}
		if bulkMessage.Msg == nil {
			var err error
			if bulkMessage, err = types.PrepareBulkMessage(&resp); err != nil {
				return err
			}
		}
		client.EnsureQueueMessage(bulkMessage)
	}
	return nil
}

func getRequiredCapabilityForMessage(message string) types.CapabilityComponent {
	switch message {
	case
		// File Tree events
		"receiveNewDoc",
		"receiveNewFile",
		"receiveNewFolder",
		"receiveEntityMove",
		"receiveEntityRename",
		"removeEntity",

		// Core project details
		"projectNameUpdated",
		"rootDocUpdated",
		"toggle-track-changes",

		// Updates
		"otUpdateError",
		"otUpdateApplied",

		// Project deleted
		"projectRenamedOrDeletedByExternalSource",

		// Auth
		"project:publicAccessLevel:changed",

		// System
		"forceDisconnect",
		"unregisterServiceWorker":
		return types.CanSeeNonRestrictedEvents
	default:
		return types.CanSeeAllEditorEvents
	}
}
