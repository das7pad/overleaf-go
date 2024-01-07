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
	"log"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

func (r *room) Handle(raw string) {
	if r.isEmpty() {
		return
	}
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
	case "project:publicAccessLevel:changed":
		err = r.handleProjectPublicAccessLevelChanged(msg)
	case "project:membership:changed":
		err = r.handleProjectMembershipChanged(msg)
	default:
		err = r.handleMessage(msg)
	}
	if err != nil {
		log.Printf(
			"%s: handle editorEvents message: %s: %s",
			msg.RoomId, msg.Message, err,
		)
		return
	}
}

type publicAccessLevelChangedPayload struct {
	NewAccessLevel project.PublicAccessLevel `json:"newAccessLevel"`
}

func (r *room) handleProjectPublicAccessLevelChanged(msg sharedTypes.EditorEventsMessage) error {
	p := publicAccessLevelChangedPayload{}
	if err := json.Unmarshal(msg.Payload, &p); err != nil {
		return errors.Tag(err, "deserialize payload")
	}
	if p.NewAccessLevel == project.PrivateAccess {
		clients := r.Clients()
		for i, client := range clients.All {
			if i == clients.Removed {
				continue
			}
			if !client.HasCapability(types.CanSeeOtherClients) {
				// This is a restricted user aka a token user who lost access.
				client.TriggerDisconnect()
			}
		}
	}
	return r.handleMessage(msg)
}

type projectMembershipChangedPayload struct {
	UserId sharedTypes.UUID `json:"userId"`
}

func (r *room) handleProjectMembershipChanged(msg sharedTypes.EditorEventsMessage) error {
	p := projectMembershipChangedPayload{}
	if err := json.Unmarshal(msg.Payload, &p); err != nil {
		return errors.Tag(err, "deserialize payload")
	}
	clients := r.Clients()
	for i, client := range clients.All {
		if i == clients.Removed {
			continue
		}
		if p.UserId == client.UserId {
			client.TriggerDisconnect()
		}
	}
	return r.handleMessage(msg)
}

func (r *room) handleMessage(msg sharedTypes.EditorEventsMessage) error {
	var requiredCapability types.CapabilityComponent
	var bulkMessage types.WriteQueueEntry
	clients := r.Clients()
	for i, client := range clients.All {
		if i == clients.Removed {
			continue
		}
		if client.PublicId == msg.Source {
			continue
		}
		if requiredCapability == 0 {
			requiredCapability = getRequiredCapabilityForMessage(msg.Message)
		}
		if !client.HasCapability(requiredCapability) {
			continue
		}
		if bulkMessage.Msg == nil {
			resp := types.RPCResponse{
				Name:        msg.Message,
				Body:        msg.Payload,
				ProcessedBy: msg.ProcessedBy,
			}
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
		"compilerUpdated",
		"imageNameUpdated",
		"projectNameUpdated",
		"rootDocUpdated",

		// Updates
		"otUpdateError",
		"otUpdateApplied",

		// Auth
		"project:publicAccessLevel:changed",

		// System
		"forceDisconnect":
		return types.CanSeeNonRestrictedEvents
	default:
		return types.CanSeeAllEditorEvents
	}
}
