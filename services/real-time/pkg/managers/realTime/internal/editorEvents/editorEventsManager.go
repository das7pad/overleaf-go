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
	GetClients() LazyRoomClients
	Join(ctx context.Context, client *types.Client) error
	Leave(client *types.Client)
	StartListening(ctx context.Context) error
}

type LazyRoomClients map[sharedTypes.UUID]*room

type FlushProject func(ctx context.Context, projectId sharedTypes.UUID) bool

func New(c channel.Manager, flushRoomChanges FlushRoomChanges, flushProject FlushProject) Manager {
	m := manager{
		c:                c,
		sem:              make(chan struct{}, 1),
		mux:              sync.RWMutex{},
		rooms:            make(map[sharedTypes.UUID]*room),
		idle:             make(map[sharedTypes.UUID]bool),
		flushRoomChanges: flushRoomChanges,
		flushProject:     flushProject,
	}
	return &m
}

type manager struct {
	c channel.Manager

	sem              chan struct{}
	mux              sync.RWMutex
	rooms            map[sharedTypes.UUID]*room
	idle             map[sharedTypes.UUID]bool
	flushRoomChanges FlushRoomChanges
	flushProject     FlushProject
}

func (m *manager) pauseQueueFor(fn func()) {
	m.sem <- struct{}{}
	fn()
	<-m.sem
}

func (m *manager) cleanup(r *room, id sharedTypes.UUID) {
	if !r.isEmpty() {
		// Someone else joined again.
		return
	}

	m.sem <- struct{}{}
	defer func() { <-m.sem }()

	if len(r.clients.All) != 0 {
		// Someone else joined while we acquired the sem.
		return
	}

	// Get write lock while we are removing the empty room.
	m.mux.Lock()
	delete(m.rooms, id)
	m.mux.Unlock()

	r.close()
	delete(m.idle, id)
}

func (m *manager) joinLocked(ctx context.Context, client *types.Client) (*room, pendingOperation.PendingOperation) {
	projectId := client.ProjectId
	// There is no need for read locking, we are the only potential writer.
	r, exists := m.rooms[projectId]
	if !exists {
		r = newRoom(projectId, m.flushRoomChanges, m.flushProject)
		m.mux.Lock()
		m.rooms[projectId] = r
		m.mux.Unlock()
	}

	roomWasEmpty := r.add(client)
	if exists && roomWasEmpty {
		if m.idle[projectId] {
			roomWasEmpty = false
		}
		delete(m.idle, projectId)
	}

	if !roomWasEmpty {
		if r.pending == nil {
			return r, nil // Long finished subscribing
		} else if err := r.pending.Err(); err == nil {
			r.pending = nil
			return r, nil // Finished subscribing
		} else if err == pendingOperation.ErrOperationStillPending {
			return r, r.pending // Subscribe is still pending
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
	return r, op
}

func (m *manager) leaveLocked(client *types.Client) *room {
	r := m.rooms[client.ProjectId]

	if turnedEmpty := r.remove(client); turnedEmpty {
		m.idle[client.ProjectId] = true
	}
	return r
}

func (m *manager) cleanupIdleRooms(ctx context.Context, threshold int) int {
	m.sem <- struct{}{}
	n := len(m.idle)
	if n < threshold {
		<-m.sem
		return n
	}
	var ids sharedTypes.UUIDBatch
	extra := 1000
	for ids.Cap() < n {
		<-m.sem
		n += extra
		extra = extra * 2
		ids = sharedTypes.NewUUIDBatch(n)

		m.sem <- struct{}{}
		n = len(m.idle)
		if n < threshold {
			<-m.sem
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
	<-m.sem
	_ = pending.Wait(ctx)
	return n
}

func (m *manager) Join(ctx context.Context, client *types.Client) error {
	m.sem <- struct{}{}
	r, pending := m.joinLocked(ctx, client)
	g := <-r.roomChanges
	<-m.sem
	r.scheduleRoomChange(client, true, g)
	if pending == nil {
		return nil
	}
	<-pending.Done()
	return pending.Err()
}

func (m *manager) Leave(client *types.Client) {
	m.sem <- struct{}{}
	r := m.leaveLocked(client)
	g := <-r.roomChanges
	<-m.sem
	r.scheduleRoomChange(client, false, g)
}

func (m *manager) handleMessage(message channel.PubSubMessage) {
	m.mux.RLock()
	r, exists := m.rooms[message.Channel]
	m.mux.RUnlock()
	if !exists {
		return
	}
	if len(message.Msg) == 0 {
		m.cleanup(r, message.Channel)
	} else {
		r.broadcast(message.Msg)
	}
}

func (m *manager) processAllMessages(allQueue <-chan string) {
	for message := range allQueue {
		msg := message
		m.pauseQueueFor(func() {
			for _, r := range m.rooms {
				r.broadcast(msg)
			}
		})
	}
}

func (m *manager) StartListening(ctx context.Context) error {
	c, err := m.c.Listen(ctx)
	if err != nil {
		return errors.Tag(err, "listen on all channel")
	}

	allQueue := make(chan string)
	go m.processAllMessages(allQueue)
	t := time.NewTicker(10 * time.Millisecond)
	go m.listen(c, allQueue, t)
	go m.periodicallyCleanupIdleRooms(ctx, t)
	return nil
}

func (m *manager) listen(c <-chan channel.PubSubMessage, allQueue chan string, t *time.Ticker) {
	defer close(allQueue)
	for msg := range c {
		if msg.Channel.IsZero() {
			allQueue <- msg.Msg
		} else {
			m.handleMessage(msg)
		}
	}
	t.Stop()
}

func (m *manager) periodicallyCleanupIdleRooms(ctx context.Context, t *time.Ticker) {
	const initialThreshold = 100
	threshold := initialThreshold
	for range t.C {
		n := m.cleanupIdleRooms(ctx, threshold)
		if n < threshold && n != 0 {
			threshold -= n
		} else {
			threshold = initialThreshold
		}
	}
}

func (m *manager) BroadcastGracefulReconnect(suffix uint8) int {
	m.sem <- struct{}{}
	n := len(m.rooms)
	for _, r := range m.rooms {
		r.broadcastGracefulReconnect(suffix)
	}
	<-m.sem
	return n
}

func (m *manager) GetClients() LazyRoomClients {
	n := 0
	m.pauseQueueFor(func() {
		n = len(m.rooms)
	})
	clients := make(LazyRoomClients, n+1000)
	m.pauseQueueFor(func() {
		for id, r := range m.rooms {
			clients[id] = r
		}
	})
	return clients
}
