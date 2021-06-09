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

	"github.com/das7pad/real-time/pkg/errors"
	"github.com/das7pad/real-time/pkg/managers/realTime/internal/channel"
	"github.com/das7pad/real-time/pkg/managers/realTime/internal/pendingOperation"
	"github.com/das7pad/real-time/pkg/types"
)

type Broadcaster interface {
	Join(ctx context.Context, client *types.Client, id primitive.ObjectID) error
	Leave(client *types.Client, id primitive.ObjectID) error
	Walk(id primitive.ObjectID, fn func(client *types.Client) error) error
}

func New(ctx context.Context, get GetNextFn, set SetNextFn, c channel.Manager) Broadcaster {
	b := &broadcaster{
		c:       c,
		getNext: get,
		setNext: set,
		queue:   make(chan action),
		mux:     sync.RWMutex{},
		rooms:   make(map[primitive.ObjectID]*room),
	}
	go b.processQueue(ctx)
	return b
}

type GetNextFn func(client *types.Client) *types.Client
type SetNextFn func(client *types.Client, next *types.Client)

type broadcaster struct {
	c channel.Manager

	getNext GetNextFn
	setNext SetNextFn

	queue chan action
	mux   sync.RWMutex
	rooms map[primitive.ObjectID]*room
}

type room struct {
	head *types.Client

	pendingSubscribe   pendingOperation.WithCancel
	pendingUnsubscribe pendingOperation.WithCancel
}

type operation int

const (
	cleanup = operation(0)
	join    = operation(1)
	leave   = operation(2)
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
	if r.head != nil {
		// Someone else joined again.
		return
	}
	delete(b.rooms, id)
}

func (b *broadcaster) join(a action) pendingOperation.WithCancel {
	// No need for read locking, we are the only potential writer.
	r, exists := b.rooms[a.id]
	var roomWasEmpty bool
	if exists {
		if r.head == nil {
			roomWasEmpty = true
		} else {
			roomWasEmpty = false

			found := false
			_ = b.walkFrom(r.head, func(client *types.Client) error {
				if client == a.client {
					found = true
					return CancelWalk
				}
				return nil
			})
			if found {
				return nil
			}
		}

		// Adding the client to the head of the clients ensures that Walk
		//  will not see this client twice.
		b.setNext(a.client, r.head)
		r.head = a.client
	} else {
		roomWasEmpty = true
		r = &room{
			head: a.client,
		}

		b.mux.Lock()
		b.rooms[a.id] = r
		b.mux.Unlock()
	}

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
	if r.head == a.client {
		r.head = b.getNext(r.head)
	} else {
		head, next := r.head, b.getNext(r.head)
		for next != nil {
			if next == a.client {
				b.setNext(head, b.getNext(a.client))
				break
			}
			head, next = next, b.getNext(next)
		}
	}
	if roomIsEmpty := r.head == nil; roomIsEmpty {
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

var CancelWalk = errors.New("cancel walk")

type WalkFn = func(client *types.Client) error

func (b *broadcaster) Walk(id primitive.ObjectID, fn func(client *types.Client) error) error {
	return b.walkFrom(b.getHead(id), fn)
}

func (b *broadcaster) walkFrom(head *types.Client, fn WalkFn) error {
	for ; head != nil; head = b.getNext(head) {
		err := fn(head)
		switch err {
		case nil:
			continue
		case CancelWalk:
			break
		default:
			return err
		}
	}
	return nil
}

func (b *broadcaster) getHead(id primitive.ObjectID) *types.Client {
	b.mux.RLock()
	defer b.mux.RUnlock()
	r, exists := b.rooms[id]
	if !exists {
		return nil
	}
	return r.head
}
