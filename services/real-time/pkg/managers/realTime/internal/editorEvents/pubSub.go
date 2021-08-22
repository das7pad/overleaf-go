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
	"context"
	"encoding/json"
	"log"
	"math/rand"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/managers/realTime/internal/broadcaster"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/managers/realTime/internal/clientTracking"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

type ProjectRoom struct {
	*broadcaster.TrackingRoom

	nextClientRefresh  time.Time
	nextProjectRefresh time.Time
	periodicRefresh    *time.Ticker
	clientTracking     clientTracking.Manager
}

const (
	ClientTrackingRefresh       = "clientTracking.refresh"
	ClientTrackingClientUpdated = "clientTracking.clientUpdated"
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
	switch msg.Message {
	case ClientTrackingRefresh:
		err = r.refreshClientPositions()
	case ClientTrackingClientUpdated:
		err = r.handleClientTrackingUpdated(&msg)
	default:
		err = r.handleMessage(&msg)
	}
	if err != nil {
		log.Println("cannot handle appliedOps message: " + err.Error())
		return
	}
}

func (r *ProjectRoom) StopPeriodicTasks() {
	t := r.periodicRefresh
	if t != nil {
		t.Stop()
	}
}

func (r *ProjectRoom) StartPeriodicTasks() {
	jitter := time.Duration(rand.Int63n(int64(30 * time.Second)))
	baseInter := clientTracking.RefreshUserEvery - jitter

	t := time.NewTicker(baseInter)
	r.periodicRefresh = t
	go func() {
		failedAttempts := 0
		for range t.C {
			err := r.refreshClientPositions()
			if err != nil {
				if failedAttempts == 0 {
					// Retry soon.
					t.Reset(baseInter / 3)
				}
				failedAttempts++
				if failedAttempts%10 == 0 {
					err = errors.Tag(
						err, "repeatedly failed to refresh clients",
					)
					log.Println(err.Error())
				}
				continue
			}
			if failedAttempts > 0 {
				failedAttempts = 0
				t.Reset(baseInter)
			}
		}
	}()
}

func (r *ProjectRoom) refreshClientPositions() error {
	t := time.Now()
	if t.Before(r.nextClientRefresh) {
		return nil
	}
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	refreshProjectExpiry := t.After(r.nextProjectRefresh)
	err := r.clientTracking.RefreshClientPositions(ctx, r.Clients(), refreshProjectExpiry)
	if err != nil {
		return err
	}
	r.nextClientRefresh = t.Add(clientTracking.RefreshUserEvery)
	r.nextProjectRefresh = t.Add(clientTracking.RefreshProjectEvery)
	return nil
}
func (r *ProjectRoom) handleClientTrackingUpdated(msg *types.EditorEventsMessage) error {
	var notification types.ClientPositionUpdateNotification
	if err := json.Unmarshal(msg.Payload, &notification); err != nil {
		return errors.Tag(err, "cannot decode notification")
	}
	return r.handleMessageFromSource(msg, notification.Source)
}

func (r *ProjectRoom) handleMessage(msg *types.EditorEventsMessage) error {
	return r.handleMessageFromSource(msg, "")
}

func (r *ProjectRoom) handleMessageFromSource(msg *types.EditorEventsMessage, id sharedTypes.PublicId) error {
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
		if client.PublicId == id {
			continue
		}
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
