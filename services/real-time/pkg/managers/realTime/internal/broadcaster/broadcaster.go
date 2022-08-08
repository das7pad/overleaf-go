// Golang port of Overleaf
// Copyright (C) 2021-2022 Jakob Ackermann <das7pad@outlook.com>
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

package broadcaster

import (
	"context"
	"sync"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/pendingOperation"
	"github.com/das7pad/overleaf-go/pkg/pubSub/channel"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/events"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

type Broadcaster interface {
	Join(ctx context.Context, client *types.Client, id sharedTypes.UUID) error
	Leave(client *types.Client, id sharedTypes.UUID) error
	StartListening(ctx context.Context) error
	TriggerGracefulReconnect() int
}

type NewRoom func(room *TrackingRoom) Room

func New(c channel.Manager, newRoom NewRoom) Broadcaster {
	b := &broadcaster{
		c:        c,
		newRoom:  newRoom,
		allQueue: make(chan *channel.PubSubMessage),
		queue:    make(chan action),
		mux:      sync.RWMutex{},
		rooms:    make(map[sharedTypes.UUID]Room),
	}
	return b
}

type broadcaster struct {
	c channel.Manager

	allQueue chan *channel.PubSubMessage

	newRoom NewRoom
	queue   chan action
	mux     sync.RWMutex
	rooms   map[sharedTypes.UUID]Room
}

type operation int

const (
	cleanup operation = iota
	join
	leave
	pause
)

type onDone chan pendingOperation.PendingOperation

type action struct {
	operation operation
	id        sharedTypes.UUID
	ctx       context.Context
	client    *types.Client
	onDone    onDone
}

//goland:noinspection SpellCheckingInspection
const hexChars = "0123456789abcdef"

func (b *broadcaster) TriggerGracefulReconnect() int {
	total := 0
	for _, c := range hexChars {
		suffix := uint8(c)
		n := 0
		b.pauseQueueFor(func() {
			for _, r := range b.rooms {
				for _, client := range r.Clients() {
					// The last character is a random hex char.
					if client.PublicId[32] != suffix {
						continue
					}
					n++
					_ = client.QueueMessage(events.ReconnectGracefullyPrepared)
				}
			}
		})
		total += n
		if n > 100 {
			// Estimate > 1600 clients.
			// Worst case for the shutdown is ~2min per full cycle.
			time.Sleep(10 * time.Second)
		} else if n > 10 {
			// Estimate 160 < total < 1600 clients.
			// Worst case for the shutdown is ~2s per full cycle.
			time.Sleep(100 * time.Millisecond)
		}
	}
	return total
}

func (b *broadcaster) pauseQueueFor(fn func()) {
	done := make(onDone)
	defer close(done)
	a := action{
		operation: pause,
		onDone:    done,
	}
	b.queue <- a
	<-done
	fn()
	go b.processQueue()
}

func (b *broadcaster) processQueue() {
	for a := range b.queue {
		switch a.operation {
		case cleanup:
			b.cleanup(a.id)
		case join:
			a.onDone <- b.join(a)
		case leave:
			a.onDone <- b.leave(a)
		case pause:
			a.onDone <- nil
			return
		}
	}
}

func (b *broadcaster) cleanup(id sharedTypes.UUID) {
	r, exists := b.rooms[id]
	if !exists {
		// Someone else cleaned it up already.
		return
	}
	if !r.isEmpty() {
		// Someone else joined again.
		return
	}

	// Get write lock while we are removing the empty room.
	b.mux.Lock()
	delete(b.rooms, id)
	b.mux.Unlock()
	r.StopPeriodicTasks()
	r.close()
}

type roomQueueEntry struct {
	msg           string
	leavingClient *types.Client
}

func (b *broadcaster) createNewRoom() Room {
	c := make(chan roomQueueEntry)
	tr := &TrackingRoom{c: c}
	tr.clients.Store(&noClients)
	r := b.newRoom(tr)
	go func() {
		for entry := range c {
			if entry.leavingClient != nil {
				entry.leavingClient.RemoveWriter()
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

func (b *broadcaster) join(a action) pendingOperation.WithCancel {
	// No need for read locking, we are the only potential writer.
	r, exists := b.rooms[a.id]
	if !exists {
		r = b.createNewRoom()
		b.mux.Lock()
		b.rooms[a.id] = r
		b.mux.Unlock()
	}

	roomWasEmpty := r.isEmpty()
	r.add(a.client)

	pending := r.pendingOperation()
	if !roomWasEmpty && (pending.IsPending() || !pending.Failed()) {
		// Already subscribed or subscribe is still pending.
		return pending
	}

	op := pendingOperation.TrackOperationWithCancel(
		a.ctx,
		func(ctx context.Context) error {
			if pending != nil && pending.IsPending() {
				pending.Cancel()
				_ = pending.Wait(ctx)
			}
			return b.c.Subscribe(ctx, a.id)
		})
	r.setPendingOperation(op)
	return op
}

func (b *broadcaster) leave(a action) pendingOperation.WithCancel {
	// No need for read locking, we are the only potential writer.
	r, exists := b.rooms[a.id]
	if !exists {
		// Already left.
		return nil
	}
	if r.isEmpty() {
		// Already left.
		return nil
	}

	r.remove(a.client)

	if !r.isEmpty() {
		// Do not unsubscribe yet.
		return nil
	}

	subscribe := r.pendingOperation()
	op := pendingOperation.TrackOperationWithCancel(
		a.ctx,
		func(ctx context.Context) error {
			if subscribe != nil && subscribe.IsPending() {
				subscribe.Cancel()
				_ = subscribe.Wait(ctx)
			}
			return b.c.Unsubscribe(ctx, a.id)
		},
	)
	r.setPendingOperation(op)
	return op
}

func (b *broadcaster) Join(ctx context.Context, client *types.Client, id sharedTypes.UUID) error {
	return b.doJoinLeave(ctx, client, id, join)
}

func (b *broadcaster) doJoinLeave(ctx context.Context, client *types.Client, id sharedTypes.UUID, target operation) error {
	done := make(onDone)
	defer close(done)
	b.queue <- action{
		operation: target,
		id:        id,
		ctx:       ctx,
		client:    client,
		onDone:    done,
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

func (b *broadcaster) Leave(client *types.Client, id sharedTypes.UUID) error {
	return b.doJoinLeave(context.Background(), client, id, leave)
}

func (b *broadcaster) handleMessage(message *channel.PubSubMessage) {
	b.mux.RLock()
	r, exists := b.rooms[message.Channel]
	b.mux.RUnlock()
	if !exists {
		return
	}
	r.broadcast(message.Msg)
}

func (b *broadcaster) processAllMessages() {
	for message := range b.allQueue {
		b.pauseQueueFor(func() {
			for _, r := range b.rooms {
				r.broadcast(message.Msg)
			}
		})
	}
}

func (b *broadcaster) StartListening(ctx context.Context) error {
	c, err := b.c.Listen(ctx)
	if err != nil {
		return errors.Tag(err, "cannot listen on all channel")
	}

	go b.processQueue()
	go b.processAllMessages()
	go func() {
		defer close(b.allQueue)
		for raw := range c {
			switch raw.Action {
			case channel.Unsubscribed:
				b.queue <- action{
					operation: cleanup,
					id:        raw.Channel,
				}
			case channel.IncomingMessage:
				if raw.Channel == (sharedTypes.UUID{}) {
					b.allQueue <- raw
				} else {
					b.handleMessage(raw)
				}
			}
		}
	}()
	return nil
}
