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

func (r *ProjectRoom) Handle(raw string) {
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
	case clientTracking.Refresh:
		err = r.refreshClientPositions()
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

func (r *ProjectRoom) StopPeriodicTasks() {
	if t := r.periodicRefresh; t != nil {
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

func (r *ProjectRoom) handleMessage(msg sharedTypes.EditorEventsMessage) error {
	resp := types.RPCResponse{
		Name:        msg.Message,
		Body:        msg.Payload,
		ProcessedBy: msg.ProcessedBy,
	}
	bulkMessage, err := types.PrepareBulkMessage(&resp)
	if err != nil {
		return err
	}
	nonRestricted := isNonRestrictedMessage(msg.Message)
	for _, client := range r.Clients() {
		if client.PublicId == msg.Source {
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
	switch message {
	//goland:noinspection SpellCheckingInspection
	case
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
		return true
	default:
		return false
	}
}

func clientCanSeeMessage(client *types.Client, nonRestrictedMessage bool) bool {
	if nonRestrictedMessage {
		return client.HasCapability(types.CanSeeNonRestrictedEvents)
	}
	return client.HasCapability(types.CanSeeAllEditorEvents)
}
