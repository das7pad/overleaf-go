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
	"sync"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/pendingOperation"
	"github.com/das7pad/overleaf-go/pkg/pubSub/channel"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

type Manager interface {
	BroadcastGracefulReconnect(suffix uint8) int
	GetClients() map[sharedTypes.UUID]Clients
	Join(ctx context.Context, client *types.Client) error
	Leave(client *types.Client)
	StartListening(ctx context.Context) error
}

func New(c channel.Manager) Manager {
	m := manager{
		c:     c,
		sem:   make(chan struct{}, 1),
		mux:   sync.RWMutex{},
		rooms: make(map[sharedTypes.UUID]*room),
	}
	return &m
}

type manager struct {
	c channel.Manager

	sem   chan struct{}
	mux   sync.RWMutex
	rooms map[sharedTypes.UUID]*room
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

	if !r.isEmpty() {
		// Someone else joined while we acquired the sem.
		return
	}

	// Get write lock while we are removing the empty room.
	m.mux.Lock()
	delete(m.rooms, id)
	m.mux.Unlock()
	r.close()
}

func (m *manager) join(ctx context.Context, client *types.Client) pendingOperation.PendingOperation {
	projectId := client.ProjectId
	// There is no need for read locking, we are the only potential writer.
	r, exists := m.rooms[projectId]
	if !exists {
		r = newRoom()
		m.mux.Lock()
		m.rooms[projectId] = r
		m.mux.Unlock()
	}

	roomWasEmpty := r.add(client)

	pending := r.pending
	if !roomWasEmpty && (pending.IsPending() || !pending.Failed()) {
		// Already subscribed or subscribe is still pending.
		return pending
	}

	op := pendingOperation.TrackOperation(func() error {
		if pending != nil && pending.IsPending() {
			_ = pending.Wait(ctx)
		}
		return m.c.Subscribe(ctx, projectId)
	})
	r.pending = op
	return op
}

func (m *manager) leave(client *types.Client) {
	projectId := client.ProjectId
	// There is no need for read locking, we are the only potential writer.
	r, exists := m.rooms[projectId]
	if !exists || r.isEmpty() {
		// Not joined yet.
		client.CloseWriteQueue()
		return
	}

	roomIsEmpty := r.remove(client)
	if !roomIsEmpty {
		// Do not unsubscribe yet.
		return
	}

	subscribe := r.pending
	r.pending = pendingOperation.TrackOperation(func() error {
		if subscribe != nil && subscribe.IsPending() {
			_ = subscribe.Wait(context.Background())
		}
		m.c.Unsubscribe(context.Background(), projectId)
		return nil
	})
}

func (m *manager) Join(ctx context.Context, client *types.Client) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case m.sem <- struct{}{}:
		pending := m.join(ctx, client)
		<-m.sem
		if pending == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-pending.Done():
			return pending.Err()
		}
	}
}

func (m *manager) Leave(client *types.Client) {
	m.sem <- struct{}{}
	m.leave(client)
	<-m.sem
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
	go func() {
		defer close(allQueue)
		for msg := range c {
			if msg.Channel.IsZero() {
				allQueue <- msg.Msg
			} else {
				m.handleMessage(msg)
			}
		}
	}()
	return nil
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

func (m *manager) GetClients() map[sharedTypes.UUID]Clients {
	n := 0
	m.pauseQueueFor(func() {
		n = len(m.rooms)
	})
	clients := make(map[sharedTypes.UUID]Clients, n+1000)
	m.pauseQueueFor(func() {
		for id, r := range m.rooms {
			clients[id] = r.Clients()
		}
	})
	return clients
}
