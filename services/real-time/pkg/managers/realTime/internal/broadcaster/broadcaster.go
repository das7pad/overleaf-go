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

package broadcaster

import (
	"context"
	"sync"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/real-time/pkg/managers/realTime/internal/channel"
	"github.com/das7pad/real-time/pkg/managers/realTime/internal/pendingOperation"
	"github.com/das7pad/real-time/pkg/types"
)

type Broadcaster interface {
	Join(ctx context.Context, client *types.Client, id primitive.ObjectID) error
	Leave(client *types.Client, id primitive.ObjectID) error
}

type NewRoom func(room *TrackingRoom) Room

func New(ctx context.Context, c channel.Manager, newRoom NewRoom) Broadcaster {
	b := &broadcaster{
		c:        c,
		newRoom:  newRoom,
		allQueue: make(chan *channel.PubSubMessage),
		queue:    make(chan action),
		mux:      sync.RWMutex{},
		rooms:    make(map[primitive.ObjectID]Room),
	}
	go b.processQueue(ctx)
	go b.listen(ctx)
	return b
}

type broadcaster struct {
	c channel.Manager

	allQueue chan *channel.PubSubMessage

	newRoom NewRoom
	queue   chan action
	mux     sync.RWMutex
	rooms   map[primitive.ObjectID]Room
}

type operation int

const (
	cleanup operation = iota
	join
	leave
)

type onDone chan pendingOperation.PendingOperation

type action struct {
	operation operation
	id        primitive.ObjectID
	ctx       context.Context
	client    *types.Client
	onDone    onDone
}

func (b *broadcaster) processQueue(ctx context.Context) {
	done := ctx.Done()
	for {
		select {
		case <-done:
			return
		case a := <-b.queue:
			switch a.operation {
			case cleanup:
				b.cleanup(a.id)
			case join:
				a.onDone <- b.join(a)
			case leave:
				a.onDone <- b.leave(a)
			}
		}
	}
}

func (b *broadcaster) cleanup(id primitive.ObjectID) {
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
	c := make(chan *roomQueueEntry)
	r := b.newRoom(&TrackingRoom{
		c:       c,
		clients: noClients,
	})
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

func (b *broadcaster) Join(ctx context.Context, client *types.Client, id primitive.ObjectID) error {
	return b.doJoinLeave(ctx, client, id, join)
}

func (b *broadcaster) doJoinLeave(ctx context.Context, client *types.Client, id primitive.ObjectID, target operation) error {
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

func (b *broadcaster) Leave(client *types.Client, id primitive.ObjectID) error {
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

func (b *broadcaster) handleAllMessage(message *channel.PubSubMessage) {
	b.mux.RLock()
	rooms := make([]Room, len(b.rooms))
	i := 0
	for _, r := range b.rooms {
		rooms[i] = r
		i++
	}
	b.mux.RUnlock()
	for _, r := range rooms {
		r.broadcast(message.Msg)
	}
}

func (b *broadcaster) processAllMessages() {
	for message := range b.allQueue {
		b.handleAllMessage(message)
	}
}

func (b *broadcaster) listen(ctx context.Context) {
	go b.processAllMessages()
	defer close(b.allQueue)
	for raw := range b.c.Listen(ctx) {
		switch raw.Action {
		case channel.Unsubscribed:
			b.queue <- action{
				operation: cleanup,
				id:        raw.Channel,
			}
		case channel.Message:
			if raw.Channel == primitive.NilObjectID {
				b.allQueue <- raw
			} else {
				b.handleMessage(raw)
			}
		}
	}
}
