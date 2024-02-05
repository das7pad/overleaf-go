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
	"context"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/das7pad/overleaf-go/pkg/pendingOperation"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/events"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

type RoomChange struct {
	PublicId    sharedTypes.PublicId `json:"i"`
	DisplayName string               `json:"n,omitempty"`
	IsJoin      bool                 `json:"j,omitempty"`
}
type RoomChanges []RoomChange
type FlushRoomChanges func(projectId sharedTypes.UUID, rc RoomChanges)

func newRoom(projectId sharedTypes.UUID, flushRoomChanges FlushRoomChanges, flushProject FlushProject) *room {
	c := make(chan roomQueueEntry, 20)
	rc := make(chan RoomChanges, 1)
	rc <- nil
	r := room{
		c:           c,
		roomChanges: rc,
	}
	r.clients.Store(noClients)
	r.roomChangesFlush = time.AfterFunc(24*time.Hour, r.queueFlushRoomChanges)
	go r.process(c, projectId, flushRoomChanges, flushProject)
	return &r
}

func (r *room) queueFlushRoomChanges() {
	r.c <- roomQueueEntry{action: actionFlushRoomChanges}
}

func (r *room) process(c chan roomQueueEntry, projectId sharedTypes.UUID, flushRoomChanges FlushRoomChanges, flushProject FlushProject) {
	for entry := range c {
		switch entry.action {
		case actionsHandleMessage:
			r.Handle(entry.msg)
		case actionFlushRoomChanges:
			r.flushRoomChanges(projectId, flushRoomChanges)
		default:
			r.handleGracefulReconnect(entry.action)
		}
	}
	flushProject(context.Background(), projectId)
}

const (
	actionsHandleMessage   = 0
	actionFlushRoomChanges = 1
)

type roomQueueEntry struct {
	action uint8
	msg    string
}

type Clients struct {
	All     types.Clients
	Removed int
}

func (c Clients) String() string {
	s := "all=["
	for i, client := range c.All {
		if i > 0 {
			s += ", "
		}
		s += client.String()
	}
	s += "] removed=" + strconv.FormatInt(int64(c.Removed), 10)
	return s
}

type room struct {
	clients          atomic.Pointer[Clients]
	c                chan roomQueueEntry
	roomChanges      chan RoomChanges
	roomChangesFlush *time.Timer

	pending pendingOperation.PendingOperation
}

var noClients = &Clients{Removed: -1}

func (r *room) Clients() Clients {
	return *r.clients.Load()
}

func (r *room) broadcast(msg string) {
	r.c <- roomQueueEntry{action: actionsHandleMessage, msg: msg}
}

func (r *room) broadcastGracefulReconnect(suffix uint8) {
	r.c <- roomQueueEntry{action: suffix}
}

func (r *room) close() {
	r.roomChangesFlush.Reset(0)
}

func (r *room) isEmpty() bool {
	return r.clients.Load() == noClients
}

func (r *room) add(client *types.Client) bool {
	defer r.scheduleRoomChange(client, true)
	p := r.clients.Load()
	if p == noClients {
		r.clients.Store(&Clients{All: []*types.Client{client}, Removed: -1})
		return true
	}
	clients := *p
	clients.All = append(clients.All, client)
	r.clients.Store(&clients)
	return false
}

func (r *room) remove(client *types.Client) bool {
	p := r.clients.Load()
	if p == noClients {
		return true
	}
	idx := p.All.Index(client)
	if idx == -1 {
		// Not found.
		return false
	}

	defer r.scheduleRoomChange(client, false)
	n := len(p.All)
	if n == 1 || (n == 2 && p.Removed != -1 && p.Removed != idx) {
		r.clients.Store(noClients)
		return true
	}

	clients := *p
	if p.Removed == -1 {
		clients.Removed = idx
	} else {
		f := make(types.Clients, n-2, n)
		copy(f, clients.All[:n-2])
		if idx < n-2 {
			if clients.Removed == n-1 {
				f[idx] = clients.All[n-2]
			} else {
				f[idx] = clients.All[n-1]
			}
		}
		if clients.Removed < n-2 {
			if idx == n-1 || idx < n-2 {
				f[clients.Removed] = clients.All[n-2]
			} else {
				f[clients.Removed] = clients.All[n-1]
			}
		}
		clients.All = f
		clients.Removed = -1
	}
	r.clients.Store(&clients)
	return false
}

func (r *room) scheduleRoomChange(client *types.Client, isJoin bool) {
	rcs := <-r.roomChanges
	owner := rcs == nil
	if owner {
		rcs = make(RoomChanges, 0, 4)
	}
	rc := RoomChange{
		PublicId: client.PublicId,
		IsJoin:   isJoin,
	}
	if isJoin {
		rc.DisplayName = client.DisplayName
	}
	rcs = append(rcs, rc)
	r.roomChanges <- rcs
	if owner {
		r.roomChangesFlush.Reset(10 * time.Millisecond)
	}
}

func (r *room) handleGracefulReconnect(suffix uint8) {
	clients := r.Clients()
	for i, client := range clients.All {
		if i == clients.Removed {
			continue
		}
		// The last character of a PublicId is a random hex char.
		if client.PublicId[21] != suffix {
			continue
		}
		client.EnsureQueueMessage(events.ReconnectGracefullyPrepared)
	}
}

func (r *room) flushRoomChanges(projectId sharedTypes.UUID, fn FlushRoomChanges) {
	rc := <-r.roomChanges
	if rc == nil {
		close(r.c)
		close(r.roomChanges)
		return
	}
	r.roomChanges <- nil
	go fn(projectId, rc)
}
