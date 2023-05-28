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
	GetClients() map[sharedTypes.UUID]Clients
	Join(ctx context.Context, client *types.Client) error
	Leave(client *types.Client) error
	StartListening(ctx context.Context) error
}

func New(c channel.Manager) Manager {
	b := &manager{
		c:        c,
		allQueue: make(chan string),
		queue:    make(chan func()),
		mux:      sync.RWMutex{},
		rooms:    make(map[sharedTypes.UUID]*room),
	}
	return b
}

type manager struct {
	c channel.Manager

	allQueue chan string

	queue chan func()
	mux   sync.RWMutex
	rooms map[sharedTypes.UUID]*room
}

func (m *manager) pauseQueueFor(fn func()) {
	done := make(chan struct{})
	m.queue <- func() {
		fn()
		close(done)
	}
	<-done
}

func (m *manager) processQueue() {
	for fn := range m.queue {
		fn()
	}
}

func (m *manager) queueCleanup(id sharedTypes.UUID) {
	m.queue <- func() {
		m.cleanup(id)
	}
}

func (m *manager) cleanup(id sharedTypes.UUID) {
	r, exists := m.rooms[id]
	if !exists {
		// Someone else cleaned it up already.
		return
	}
	if !r.isEmpty() {
		// Someone else joined again.
		return
	}

	// Get write lock while we are removing the empty room.
	m.mux.Lock()
	delete(m.rooms, id)
	m.mux.Unlock()
	r.close()
}

type roomQueueEntry struct {
	msg           string
	leavingClient *types.Client
}

func (m *manager) createNewRoom() *room {
	c := make(chan roomQueueEntry, 10)
	r := &room{c: c}
	r.clients.Store(noClients)
	go func() {
		for entry := range c {
			if entry.leavingClient != nil {
				entry.leavingClient.CloseWriteQueue()
				continue
			}

			if r.isEmpty() {
				continue
			}
			r.Handle(entry.msg)
		}
	}()
	return r
}

func (m *manager) join(ctx context.Context, client *types.Client) pendingOperation.WithCancel {
	projectId := client.ProjectId
	// There is no need for read locking, we are the only potential writer.
	r, exists := m.rooms[projectId]
	if !exists {
		r = m.createNewRoom()
		m.mux.Lock()
		m.rooms[projectId] = r
		m.mux.Unlock()
	}

	roomWasEmpty := r.isEmpty()
	r.add(client)

	pending := r.pending
	if !roomWasEmpty && (pending.IsPending() || !pending.Failed()) {
		// Already subscribed or subscribe is still pending.
		return pending
	}

	op := pendingOperation.TrackOperationWithCancel(
		ctx,
		func(ctx context.Context) error {
			if pending != nil && pending.IsPending() {
				pending.Cancel()
				_ = pending.Wait(ctx)
			}
			return m.c.Subscribe(ctx, projectId)
		})
	r.pending = op
	return op
}

func (m *manager) leave(client *types.Client) pendingOperation.WithCancel {
	projectId := client.ProjectId
	// There is no need for read locking, we are the only potential writer.
	r, exists := m.rooms[projectId]
	if !exists {
		// Already left.
		client.CloseWriteQueue()
		return nil
	}
	if r.isEmpty() {
		// Already left.
		client.CloseWriteQueue()
		return nil
	}

	r.remove(client)

	if !r.isEmpty() {
		// Do not unsubscribe yet.
		return nil
	}

	subscribe := r.pending
	op := pendingOperation.TrackOperationWithCancel(
		context.Background(),
		func(ctx context.Context) error {
			if subscribe != nil && subscribe.IsPending() {
				subscribe.Cancel()
				_ = subscribe.Wait(ctx)
			}
			return m.c.Unsubscribe(ctx, projectId)
		},
	)
	r.pending = op
	return op
}

func (m *manager) Join(ctx context.Context, client *types.Client) error {
	done := make(chan pendingOperation.PendingOperation)
	defer close(done)
	m.queue <- func() {
		done <- m.join(ctx, client)
	}
	select {
	case <-ctx.Done():
		<-done
		return ctx.Err()
	case pending := <-done:
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

func (m *manager) Leave(client *types.Client) error {
	done := make(chan pendingOperation.PendingOperation)
	defer close(done)
	m.queue <- func() {
		done <- m.leave(client)
	}
	if pending := <-done; pending != nil {
		<-pending.Done()
		return pending.Err()
	}
	return nil
}

func (m *manager) handleMessage(message *channel.PubSubMessage) {
	m.mux.RLock()
	r, exists := m.rooms[message.Channel]
	m.mux.RUnlock()
	if !exists {
		return
	}
	r.broadcast(message.Msg)
}

func (m *manager) processAllMessages() {
	for message := range m.allQueue {
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

	go m.processQueue()
	go m.processAllMessages()
	go func() {
		defer close(m.allQueue)
		for raw := range c {
			switch raw.Action {
			case channel.Unsubscribed:
				m.queueCleanup(raw.Channel)
			case channel.IncomingMessage:
				if raw.Channel.IsZero() {
					m.allQueue <- raw.Msg
				} else {
					m.handleMessage(raw)
				}
			}
		}
	}()
	return nil
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
