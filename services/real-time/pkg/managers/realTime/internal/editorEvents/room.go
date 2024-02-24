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

type FlushRoomChanges func(projectId sharedTypes.UUID, rcs types.RoomChanges)

const delayFlushRoomChanges = 10 * time.Millisecond

func newRoom(projectId sharedTypes.UUID, flushRoomChanges FlushRoomChanges, flushProject FlushProject) *room {
	r := room{c: make(chan roomQueueEntry, 20), clients: &noClients}
	r.roomChangesFlush = time.AfterFunc(time.Hour, func() {
		r.flushRoomChanges(projectId, flushRoomChanges)
	})
	go r.process(projectId, flushProject)
	return &r
}

func (r *room) process(projectId sharedTypes.UUID, flushProject FlushProject) {
	for entry := range r.c {
		switch entry.action {
		case actionsHandleMessage:
			r.Handle(entry.msg)
		default:
			r.handleGracefulReconnect(entry.action)
		}
	}
	flushProject(context.Background(), projectId)
}

const (
	actionsHandleMessage = 0
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

func putClients(c *Clients) {
	for i := 0; i < len(c.All); i++ {
		c.All[i] = nil
	}
	idx, _ := getClientsPoolBucket(cap(c.All))
	clientsPool[idx].Put(c)
}

func newClients(lower int) *Clients {
	idx, upper := getClientsPoolBucket(lower)
	if v := clientsPool[idx].Get(); v != nil {
		c := v.(*Clients)
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

func newClientsSlow(lower, upper int) *Clients {
	return &Clients{
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

func (r RemovedClients) HasAfter(i int, start uint8) bool {
	j := int32(i)
	for idx := start; idx < removedClientsLen; idx++ {
		if r[idx] == j {
			return true
		}
		if r[idx] == -1 {
			return false
		}
	}
	return false
}

func (c Clients) String() string {
	var s strings.Builder
	s.WriteString(c.All.String())
	s.WriteString(" removed=[")
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

type roomChangeInc struct {
	join      uint16
	removed   uint8
	pending   bool
	dirty     bool
	closed    bool
	scheduled bool
	rcs       types.RoomChanges
}

type room struct {
	mu               sync.Mutex
	clients          *Clients
	rci              roomChangeInc
	c                chan roomQueueEntry
	roomChangesFlush *time.Timer

	pending pendingOperation.PendingOperation
}

var (
	noneRemoved = RemovedClients{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1}
	noClients   = Clients{Removed: noneRemoved}
)

func (r *room) swapClients(old, next *Clients, removed int, join uint16) bool {
	r.mu.Lock()
	r.renderRoomChanges(removed)
	r.clients = next
	r.rci.removed = 0
	r.rci.join += join
	s := r.rci.scheduled
	r.rci.scheduled = true
	r.mu.Unlock()
	old.Done()
	return s
}

func (c *Clients) Done() {
	if len(c.All) == 0 {
		return
	}
	if c.allRef.Add(-1) == -1 {
		putClients(c)
	}
}

func (r *room) Clients() Clients {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := *r.clients
	if len(c.All) > 0 {
		c.allRef.Add(1)
	}
	return c
}

func (r *room) broadcast(msg string) {
	r.c <- roomQueueEntry{action: actionsHandleMessage, msg: msg}
}

func (r *room) broadcastGracefulReconnect(suffix uint8) {
	r.c <- roomQueueEntry{action: suffix}
}

func (r *room) close() {
	r.mu.Lock()
	r.rci.closed = true
	s := r.rci.scheduled
	r.rci.scheduled = true
	r.mu.Unlock()
	if !s {
		r.roomChangesFlush.Reset(0)
	}
}

func (r *room) isEmpty() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.clients.All) == 0
}

func (r *room) add(client *types.Client) (bool, bool) {
	clients := r.clients
	if len(clients.All) == 0 {
		c := newClients(1)
		c.All = append(c.All[:0], client)
		r.mu.Lock()
		r.clients = c
		r.rci.join++
		s := r.rci.scheduled
		r.rci.scheduled = true
		r.mu.Unlock()
		return true, s
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
		return false, r.swapClients(clients, c, -1, 1)
	} else {
		r.mu.Lock()
		r.clients.All = append(clients.All, client)
		r.rci.join++
		s := r.rci.scheduled
		r.rci.scheduled = true
		r.mu.Unlock()
		return false, s
	}
}

func (r *room) remove(client *types.Client) (bool, bool) {
	clients := r.clients

	n := len(clients.All)
	m := clients.Removed.Len()
	idx := clients.All.Index(client)
	if n == m+1 {
		return true, r.swapClients(clients, &noClients, idx, 0)
	}

	if m < removedClientsLen {
		r.mu.Lock()
		r.clients.Removed[m] = int32(idx)
		s := r.rci.scheduled
		r.rci.scheduled = true
		r.mu.Unlock()
		return false, s
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
		return false, r.swapClients(clients, c, idx, 0)
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

func (r *room) scheduleFlushRoomChanges() {
	r.roomChangesFlush.Reset(delayFlushRoomChanges)
}

func (r *room) renderRoomChanges(removed int) {
	rci := r.rci
	extra := r.clients.Removed.Len() - int(r.rci.removed) + int(r.rci.join)
	rcs := rci.rcs
	if rci.pending && !rci.dirty {
		rcs = make(types.RoomChanges, 0, extra+(len(rcs)+cap(rcs))/2)
		rci.dirty = true
	}
	if cap(rcs)-len(rcs) < extra {
		rcs = make(types.RoomChanges, len(rcs), extra+(len(rcs)+cap(rcs))/2)
		copy(rcs, rci.rcs)
	}
	clients := r.clients
	n := len(clients.All)
	for i := n - int(rci.join); i < n; i++ {
		if i == removed || clients.Removed.HasAfter(i, rci.removed) {
			continue
		}
		rcs = append(rcs, types.RoomChange{
			PublicId:    clients.All[i].PublicId,
			DisplayName: clients.All[i].DisplayName,
			IsJoin:      1,
		})
	}
	for i := rci.removed; i < removedClientsLen; i++ {
		idx := clients.Removed[i]
		if idx == -1 {
			rci.removed = i
			break
		}
		rcs = append(rcs, types.RoomChange{
			PublicId: clients.All[idx].PublicId,
		})
	}
	if removed != -1 {
		rcs = append(rcs, types.RoomChange{
			PublicId: clients.All[removed].PublicId,
		})
	}
	rci.join = 0
	rci.rcs = rcs
	r.rci = rci
}

func (r *room) flushRoomChanges(projectId sharedTypes.UUID, fn FlushRoomChanges) {
	r.mu.Lock()
	r.renderRoomChanges(-1)
	r.rci.pending = true
	r.rci.dirty = false
	r.rci.scheduled = false
	closed := r.rci.closed
	rcs := r.rci.rcs
	r.mu.Unlock()
	if len(rcs) == 0 {
		if closed {
			close(r.c)
		}
		return
	}
	fn(projectId, rcs)
	r.mu.Lock()
	if len(r.rci.rcs) > 0 &&
		r.rci.rcs[0].IsJoin == rcs[0].IsJoin &&
		r.rci.rcs[0].PublicId == rcs[0].PublicId {
		r.rci.rcs = rcs[:0]
		r.rci.pending = false
		r.rci.dirty = false
	}
	r.mu.Unlock()
}
