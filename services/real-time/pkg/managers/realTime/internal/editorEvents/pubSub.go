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

package editorEvents

import (
	"encoding/json"
	"log"

	"github.com/das7pad/real-time/pkg/managers/realTime/internal/broadcaster"
	"github.com/das7pad/real-time/pkg/types"
)

type ProjectRoom struct {
	*broadcaster.TrackingRoom
}

func newRoom(room *broadcaster.TrackingRoom) broadcaster.Room {
	return &ProjectRoom{
		TrackingRoom: room,
	}
}

const (
	clientTrackingRefresh = "clientTracking.refresh"
)

var nonRestrictedMessages = []string{
	// File Tree events
	// NOTE: The actual event names have a typo.
	"reciveNewDoc",
	"reciveNewFile",
	"reciveNewFolder",
	"reciveEntityMove",
	"reciveEntityRename",
	"removeEntity",

	// Core project details
	"projectNameUpdated",
	"rootDocUpdated",
	"toggle-track-changes",

	// Project deleted
	"projectRenamedOrDeletedByExternalSource",

	// Auth
	"project:publicAccessLevel:changed",
}

func (r *ProjectRoom) Handle(raw string) {
	var msg types.EditorEventsMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		log.Println("cannot parse editorEvents message: " + err.Error())
		return
	}
	if err := msg.Validate(); err != nil {
		log.Println("cannot validate editorEvents message: " + err.Error())
		return
	}
	if msg.HealthCheck {
		return
	}
	var err error
	if msg.Message == clientTrackingRefresh {
		err = nil
	} else {
		err = r.handleMessage(&msg)
	}
	if err != nil {
		log.Println("cannot handle appliedOps message: " + err.Error())
		return
	}
}

func (r *ProjectRoom) handleMessage(msg *types.EditorEventsMessage) error {
	resp := types.RPCResponse{
		Name: msg.Message,
		Body: msg.Payload,
	}
	bulkMessage, err := types.PrepareBulkMessage(&resp)
	if err != nil {
		return err
	}
	nonRestricted := isNonRestrictedMessage(msg.Message)
	for _, client := range r.Clients() {
		if !clientCanSeeMessage(client, nonRestricted) {
			continue
		}
		client.EnsureQueueMessage(bulkMessage)
	}
	return nil
}

func isNonRestrictedMessage(message string) bool {
	for _, nonRestrictedMessage := range nonRestrictedMessages {
		if message == nonRestrictedMessage {
			return true
		}
	}
	return false
}

func clientCanSeeMessage(client *types.Client, nonRestrictedMessage bool) bool {
	if nonRestrictedMessage {
		return client.HasCapability(types.CanSeeNonRestrictedEvents)
	} else {
		return client.HasCapability(types.CanSeeAllEditorEvents)
	}
}
