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
	GetClients(id primitive.ObjectID) flatClients
}

func New(ctx context.Context, c channel.Manager) Broadcaster {
	b := &broadcaster{
		c:     c,
		queue: make(chan action),
		mux:   sync.RWMutex{},
		rooms: make(map[primitive.ObjectID]*room),
	}
	go b.processQueue(ctx)
	return b
}

type broadcaster struct {
	c channel.Manager

	queue chan action
	mux   sync.RWMutex
	rooms map[primitive.ObjectID]*room
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
	// Get write lock while we are removing the empty room.
	b.mux.Lock()
	defer b.mux.Unlock()

	r, exists := b.rooms[id]
	if !exists {
		// Someone else cleaned it up already.
		return
	}
	if !r.isEmpty() {
		// Someone else joined again.
		return
	}
	delete(b.rooms, id)
}

func (b *broadcaster) join(a action) pendingOperation.WithCancel {
	// No need for read locking, we are the only potential writer.
	r, exists := b.rooms[a.id]
	if !exists {
		r = &room{flat: flatClients{a.client}}
		b.mux.Lock()
		b.rooms[a.id] = r
		b.mux.Unlock()
	}

	roomWasEmpty := r.isEmpty()
	r.add(a.client)

	lastSubscribeFailed := r.pendingSubscribe != nil &&
		r.pendingSubscribe.Failed()
	if roomWasEmpty || lastSubscribeFailed {
		unsubscribe := r.pendingUnsubscribe
		r.pendingSubscribe = pendingOperation.TrackOperationWithCancel(
			a.ctx,
			func(ctx context.Context) error {
				if unsubscribe != nil && unsubscribe.IsPending() {
					unsubscribe.Cancel()
					_ = unsubscribe.Wait(ctx)
				}
				return b.c.Subscribe(ctx, a.id)
			},
		)
	}
	return r.pendingSubscribe
}

func (b *broadcaster) leave(a action) pendingOperation.WithCancel {
	// No need for read locking, we are the only potential writer.
	r, exists := b.rooms[a.id]
	if !exists {
		// Already left.
		return nil
	}

	r.remove(a.client)

	if r.isEmpty() {
		subscribe := r.pendingSubscribe
		r.pendingUnsubscribe = pendingOperation.TrackOperationWithCancel(
			a.ctx,
			func(ctx context.Context) error {
				if subscribe != nil && subscribe.IsPending() {
					subscribe.Cancel()
					_ = subscribe.Wait(ctx)
				}
				err := b.c.Unsubscribe(ctx, a.id)
				b.queue <- action{
					operation: cleanup,
					id:        a.id,
				}
				return err
			},
		)
		return r.pendingUnsubscribe
	}
	return nil
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

func (b *broadcaster) GetClients(id primitive.ObjectID) flatClients {
	b.mux.RLock()
	defer b.mux.RUnlock()
	r, exists := b.rooms[id]
	if !exists {
		return noClients
	}
	return r.flat
}
