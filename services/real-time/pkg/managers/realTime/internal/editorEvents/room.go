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
	"strings"
	"sync"
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

const delayFlushRoomChanges = 10 * time.Millisecond

func newRoom(projectId sharedTypes.UUID, flushRoomChanges FlushRoomChanges, flushProject FlushProject) *room {
	c := make(chan roomQueueEntry, 20)
	rc := make(chan RoomChanges, 1)
	rc <- make(RoomChanges, 0, 10)
	r := room{
		c:           c,
		roomChanges: rc,
	}
	r.clients = noClients
	r.roomChangesFlush = time.AfterFunc(delayFlushRoomChanges, r.queueFlushRoomChanges)
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

const clientsPoolBuckets = 11

var clientsPool [clientsPoolBuckets]sync.Pool

func getClientsPoolBucket(n int) (int, int) {
	if n == 1 || n == 2 {
		return n - 1, n
	}
	if n <= removedClientsLen {
		return (n + 1) / 2, n + n%2
	}
	x := (n + removedClientsLen - 1) / removedClientsLen
	if b := removedClientsLen/2 - 1 + x; b < clientsPoolBuckets {
		return b, x * removedClientsLen
	}
	return clientsPoolBuckets - 1, x * removedClientsLen
}

func putClients(c Clients) {
	for i := 0; i < len(c.All); i++ {
		c.All[i] = nil
	}
	idx, _ := getClientsPoolBucket(cap(c.All))
	clientsPool[idx].Put(c)
}

func newClients(lower int) Clients {
	idx, upper := getClientsPoolBucket(lower)
	if v := clientsPool[idx].Get(); v != nil {
		c := v.(Clients)
		c.allRef.Add(1)
		c.Removed = noneRemoved
		if cap(c.All) < lower || cap(c.All) > lower*2 {
			c.All = make(types.Clients, lower, upper)
		}
		return c
	} else {
		return newClientsSlow(lower, upper)
	}
}

func newClientsSlow(lower, upper int) Clients {
	return Clients{
		All:     make(types.Clients, lower, upper),
		Removed: noneRemoved,
		allRef:  &atomic.Int32{},
	}
}

type Clients struct {
	All     types.Clients
	Removed RemovedClients
	allRef  *atomic.Int32
}

const removedClientsLen = 10

type RemovedClients [removedClientsLen]int32

func (r RemovedClients) Len() int {
	if r[0] == -1 {
		return 0
	}
	if r[5] == -1 {
		if r[1] == -1 {
			return 1
		}
		if r[2] == -1 {
			return 2
		}
		if r[3] == -1 {
			return 3
		}
		if r[4] == -1 {
			return 4
		}
		return 5
	}
	if r[6] == -1 {
		return 6
	}
	if r[7] == -1 {
		return 7
	}
	if r[8] == -1 {
		return 8
	}
	if r[9] == -1 {
		return 9
	}
	return 10
}

func (r RemovedClients) Has(i int) bool {
	j := int32(i)
	return r[0] == j || r[1] == j || r[2] == j || r[3] == j || r[4] == j || r[5] == j || r[6] == j || r[7] == j || r[8] == j || r[9] == j
}

func (c Clients) String() string {
	var s strings.Builder
	s.WriteByte('[')
	for _, client := range c.All {
		if s.Len() > 1 {
			s.WriteString(", ")
		}
		s.WriteString(client.String())
	}
	s.WriteString("] removed=[")
	for i, idx := range c.Removed {
		if idx == -1 {
			continue
		}
		if i > 0 {
			s.WriteString(", ")
		}
		s.WriteString(c.All[idx].String())
	}
	s.WriteByte(']')
	return s.String()
}

type room struct {
	clients          Clients
	clientsMu        sync.Mutex
	c                chan roomQueueEntry
	roomChanges      chan RoomChanges
	roomChangesFlush *time.Timer

	pending pendingOperation.PendingOperation
}

var (
	noneRemoved = RemovedClients{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
	noClients   = Clients{Removed: noneRemoved}
)

func (r *room) swapClients(old, next Clients) {
	r.clientsMu.Lock()
	r.clients = next
	r.clientsMu.Unlock()
	old.Done()
}

func (c Clients) Done() {
	if len(c.All) == 0 {
		return
	}
	if c.allRef.Add(-1) == -1 {
		putClients(c)
	}
}

func (r *room) Clients() Clients {
	r.clientsMu.Lock()
	c := r.clients
	if len(c.All) > 0 {
		c.allRef.Add(1)
	}
	r.clientsMu.Unlock()
	return c
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
	r.clientsMu.Lock()
	defer r.clientsMu.Unlock()
	return len(r.clients.All) == 0
}

func (r *room) add(client *types.Client) bool {
	defer r.scheduleRoomChange(client, true)
	clients := r.clients
	if len(clients.All) == 0 {
		c := newClients(1)
		c.All = append(c.All[:0], client)
		r.swapClients(clients, c)
		return true
	}
	if n := len(clients.All); n == cap(clients.All) {
		m := clients.Removed.Len()
		c := newClients(n - m + 1)
		f := c.All[:n-m]
		for i, j := 0, 0; i < n; i++ {
			if i-j == m {
				copy(f[j:], clients.All[i:])
				break
			}
			if clients.Removed.Has(i) {
				continue
			}
			f[j] = clients.All[i]
			j++
		}
		c.All = append(f, client)
		r.swapClients(clients, c)
	} else {
		r.clientsMu.Lock()
		r.clients.All = append(clients.All, client)
		r.clientsMu.Unlock()
	}
	return false
}

func (r *room) remove(client *types.Client) bool {
	clients := r.clients
	idx := clients.All.Index(client)
	if idx == -1 {
		return false
	}

	defer r.scheduleRoomChange(client, false)
	n := len(clients.All)
	m := clients.Removed.Len()
	if n == m+1 {
		r.swapClients(clients, noClients)
		return true
	}

	if m < removedClientsLen {
		r.clientsMu.Lock()
		r.clients.Removed[m] = int32(idx)
		r.clientsMu.Unlock()
	} else {
		c := newClients(n - removedClientsLen - 1)
		f := c.All[:n-removedClientsLen-1]
		for i, j := 0, 0; i < n; i++ {
			if i-j == removedClientsLen+1 {
				copy(f[j:], clients.All[i:])
				break
			}
			if i == idx || clients.Removed.Has(i) {
				continue
			}
			f[j] = clients.All[i]
			j++
		}
		c.All = f
		r.swapClients(clients, c)
	}
	return false
}

func (r *room) scheduleRoomChange(client *types.Client, isJoin bool) {
	rcs := <-r.roomChanges
	owner := rcs == nil
	if owner {
		rcs = make(RoomChanges, 0, 10)
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
		r.roomChangesFlush.Reset(delayFlushRoomChanges)
	}
}

func (r *room) handleGracefulReconnect(suffix uint8) {
	clients := r.Clients()
	defer clients.Done()
	for i, client := range clients.All {
		if clients.Removed.Has(i) {
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
