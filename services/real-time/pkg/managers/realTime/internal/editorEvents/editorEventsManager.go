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
	"maps"
	"sync"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/pendingOperation"
	"github.com/das7pad/overleaf-go/pkg/pubSub/channel"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

type Manager interface {
	BroadcastGracefulReconnect(suffix uint8) int
	GetRooms() Rooms
	GetRoomsFlat() RoomsFlat
	Join(ctx context.Context, client *types.Client) error
	Leave(client *types.Client)
	StartListening(ctx context.Context) error
	StopListening()
}

type Rooms map[sharedTypes.UUID]*room
type RoomsFlat []*room

type FlushProject func(ctx context.Context, projectId sharedTypes.UUID) bool

func New(c channel.Manager, flushRoomChanges FlushRoomChanges, flushProject FlushProject) Manager {
	m := manager{
		c:                c,
		rooms:            make(map[sharedTypes.UUID]*room),
		idle:             make(map[sharedTypes.UUID]bool),
		flushRoomChanges: flushRoomChanges,
		flushProject:     flushProject,
	}
	return &m
}

type cleanupTickerState int8

const (
	cleanupTickerStopped cleanupTickerState = iota
	cleanupTickerSlow
	cleanupTickerFast
)

type manager struct {
	c channel.Manager

	sem                sync.Mutex
	roomsMux           sync.RWMutex
	rooms              map[sharedTypes.UUID]*room
	idle               map[sharedTypes.UUID]bool
	flushRoomChanges   FlushRoomChanges
	flushProject       FlushProject
	cleanupTickerMux   sync.Mutex
	cleanupTicker      *time.Ticker
	cleanupTickerState cleanupTickerState
}

func (m *manager) StopListening() {
	m.c.Close()
}

func (m *manager) cleanup(r *room, id sharedTypes.UUID) {
	if !r.isEmpty() {
		// Someone else joined again.
		return
	}

	m.sem.Lock()
	defer m.sem.Unlock()

	if len(r.clients.All) != 0 {
		// Someone else joined while we acquired the sem.
		return
	}

	// Get write lock while we are removing the empty room.
	m.roomsMux.Lock()
	delete(m.rooms, id)
	m.roomsMux.Unlock()

	r.close()
	delete(m.idle, id)
}

func (m *manager) joinLocked(ctx context.Context, r *room, exists bool, client *types.Client) (*room, pendingOperation.PendingOperation, bool) {
	projectId := client.ProjectId
	if exists {
		r.mu.Lock()
		exists = !r.rci.closed
		r.mu.Unlock()
	}
	if !exists {
		// Retry lookup under sem lock, the room might exist now.
		// We are the only potential writer, skip read-locking the roomsMux.
		r, exists = m.rooms[projectId]
	}
	if !exists {
		r = newRoom(projectId, m.flushRoomChanges, m.flushProject)
		m.roomsMux.Lock()
		m.rooms[projectId] = r
		m.roomsMux.Unlock()
	}

	roomWasEmpty, s := r.add(client)
	if exists && roomWasEmpty {
		if m.idle[projectId] {
			roomWasEmpty = false
		}
		delete(m.idle, projectId)
	}
	if !roomWasEmpty {
		if r.pending == nil {
			return r, nil, s // Long finished subscribing
		} else if err := r.pending.Err(); err == nil {
			r.pending = nil
			return r, nil, s // Finished subscribing
		} else if err == pendingOperation.ErrOperationStillPending {
			return r, r.pending, s // Subscribe is still pending
		}
		// Retry subscribing
	}

	pending := r.pending
	op := pendingOperation.TrackOperation(func() error {
		if pending != nil {
			_ = pending.Wait(ctx)
		}
		return m.c.Subscribe(ctx, projectId)
	})
	r.pending = op
	return r, op, s
}

func (m *manager) cleanupIdleRooms(ctx context.Context, threshold int) int {
	m.sem.Lock()
	n := len(m.idle)
	if n < threshold {
		m.sem.Unlock()
		return n
	}
	var ids sharedTypes.UUIDBatch
	extra := 1000
	for ids.Cap() < n {
		m.sem.Unlock()
		n += extra
		extra = extra * 2
		ids = sharedTypes.NewUUIDBatch(n)

		m.sem.Lock()
		n = len(m.idle)
		if n < threshold {
			m.sem.Unlock()
			return n
		}
	}
	for id, ok := range m.idle {
		if ok {
			ids.Add(id)
		}
	}
	pending := pendingOperation.TrackOperation(func() error {
		return m.c.UnSubscribeBulk(ctx, ids)
	})
	for id, ok := range m.idle {
		if ok {
			m.idle[id] = false
			m.rooms[id].pending = pending
		}
	}
	m.sem.Unlock()
	_ = pending.Wait(ctx)
	return n
}

func (m *manager) Join(ctx context.Context, client *types.Client) error {
	m.roomsMux.RLock()
	r, exists := m.rooms[client.ProjectId]
	m.roomsMux.RUnlock()
	m.sem.Lock()
	r, pending, s := m.joinLocked(ctx, r, exists, client)
	m.sem.Unlock()
	if !s {
		r.scheduleFlushRoomChanges()
	}
	if pending == nil {
		return nil
	}
	<-pending.Done()
	return pending.Err()
}

func (m *manager) leaveLocked(r *room, client *types.Client) bool {
	turnedEmpty, s := r.remove(client)
	if turnedEmpty {
		m.idle[client.ProjectId] = true
	}
	return s
}

func (m *manager) Leave(client *types.Client) {
	m.roomsMux.RLock()
	r := m.rooms[client.ProjectId]
	m.roomsMux.RUnlock()
	m.sem.Lock()
	s := m.leaveLocked(r, client)
	m.sem.Unlock()
	if !s {
		r.scheduleFlushRoomChanges()
	}
}

func (m *manager) handleMessage(message channel.PubSubMessage) {
	m.roomsMux.RLock()
	r, exists := m.rooms[message.Channel]
	m.roomsMux.RUnlock()
	if !exists {
		return
	}
	if len(message.Msg) == 0 {
		m.cleanup(r, message.Channel)
	} else {
		// Only m.cleanup can take away the room, so we are OK to just take the
		//  read lock at the time of getting the room.
		// m.handleMessage is called synchronous from a single loop.
		r.broadcast(message.Msg)
	}
}

func (m *manager) processAllMessages(allQueue <-chan string) {
	for message := range allQueue {
		m.roomsMux.RLock()
		for _, r := range m.rooms {
			r.broadcast(message)
		}
		m.roomsMux.RUnlock()
	}
}

func (m *manager) StartListening(ctx context.Context) error {
	c, err := m.c.Listen(ctx)
	if err != nil {
		return errors.Tag(err, "listen on all channel")
	}

	m.cleanupTickerMux.Lock()
	m.cleanupTicker = time.NewTicker(100 * time.Millisecond)
	m.cleanupTickerState = cleanupTickerSlow
	m.cleanupTickerMux.Unlock()

	allQueue := make(chan string)
	go m.processAllMessages(allQueue)
	go m.listen(c, allQueue)
	go m.periodicallyCleanupIdleRooms(ctx)
	return nil
}

func (m *manager) listen(c <-chan channel.PubSubMessage, allQueue chan string) {
	defer close(allQueue)
	for msg := range c {
		if msg.Channel.IsZero() {
			allQueue <- msg.Msg
		} else {
			m.handleMessage(msg)
		}
	}
	m.cleanupTickerMux.Lock()
	m.cleanupTickerState = cleanupTickerStopped
	m.cleanupTicker.Stop()
	m.cleanupTickerMux.Unlock()
}

func (m *manager) periodicallyCleanupIdleRooms(ctx context.Context) {
	const initialThreshold = 100
	threshold := initialThreshold
	for range m.cleanupTicker.C {
		n := m.cleanupIdleRooms(ctx, threshold)
		m.cleanupTickerMux.Lock()
		if n < threshold && m.cleanupTickerState == cleanupTickerFast {
			m.cleanupTickerState = cleanupTickerSlow
			m.cleanupTicker.Reset(100 * time.Millisecond)
		} else if n >= threshold && m.cleanupTickerState == cleanupTickerSlow {
			m.cleanupTickerState = cleanupTickerFast
			m.cleanupTicker.Reset(10 * time.Millisecond)
		}
		m.cleanupTickerMux.Unlock()
		if n < threshold && n != 0 {
			threshold -= n
		} else {
			threshold = initialThreshold
		}
	}
}

func (m *manager) BroadcastGracefulReconnect(suffix uint8) int {
	m.roomsMux.RLock()
	defer m.roomsMux.RUnlock()
	for _, r := range m.rooms {
		r.broadcastGracefulReconnect(suffix)
	}
	return len(m.rooms)
}

func (m *manager) GetRooms() Rooms {
	m.roomsMux.RLock()
	defer m.roomsMux.RUnlock()
	clients := maps.Clone(m.rooms)
	return clients
}

func (m *manager) GetRoomsFlat() RoomsFlat {
	m.roomsMux.RLock()
	defer m.roomsMux.RUnlock()
	var rooms RoomsFlat
	n := len(m.rooms)
	for ; n > cap(rooms); n = len(m.rooms) {
		m.roomsMux.RUnlock()
		rooms = make(RoomsFlat, n)
		m.roomsMux.RLock()
	}
	if n == 0 {
		return nil
	}
	rooms = rooms[:n]
	i := 0
	for _, r := range m.rooms {
		rooms[i] = r
		i++
	}
	return rooms
}
