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
	"sync/atomic"

	"github.com/das7pad/overleaf-go/pkg/pendingOperation"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

type Clients []*types.Client

var noClients = make(Clients, 0)

type Room interface {
	Handle(msg string)
	Clients() Clients
	StartPeriodicTasks()
	StopPeriodicTasks()

	isEmpty() bool
	add(client *types.Client)
	remove(client *types.Client)

	broadcast(msg string)
	close()

	pendingOperation() pendingOperation.WithCancel
	setPendingOperation(p pendingOperation.WithCancel)
}

type TrackingRoom struct {
	clients atomic.Pointer[Clients]
	c       chan roomQueueEntry

	pending pendingOperation.WithCancel
}

func (r *TrackingRoom) Handle(_ string) {
	panic("not implemented")
}

func (r *TrackingRoom) Clients() Clients {
	return *r.clients.Load()
}

func (r *TrackingRoom) StartPeriodicTasks() {
	// Noop
}
func (r *TrackingRoom) StopPeriodicTasks() {
	// Noop
}

func (r *TrackingRoom) pendingOperation() pendingOperation.WithCancel {
	return r.pending
}

func (r *TrackingRoom) setPendingOperation(p pendingOperation.WithCancel) {
	r.pending = p
}

func (r *TrackingRoom) broadcast(msg string) {
	if r.isEmpty() {
		// Safeguard for dead room.
		return
	}
	r.c <- roomQueueEntry{msg: msg}
}

func (r *TrackingRoom) close() {
	close(r.c)
}

func (r *TrackingRoom) isEmpty() bool {
	return len(*r.clients.Load()) == 0
}

func (r *TrackingRoom) add(client *types.Client) {
	clients := *r.clients.Load()
	for _, c := range clients {
		if c == client {
			return
		}
	}

	n := len(clients) + 1
	f := make(Clients, n)
	copy(f, clients)
	f[n-1] = client
	client.AddWriter()
	r.clients.Store(&f)
}

func (r *TrackingRoom) remove(client *types.Client) {
	clients := *r.clients.Load()
	idx := -1
	for i, c := range clients {
		if c == client {
			idx = i
			break
		}
	}
	if idx == -1 {
		// Not found.
		return
	}

	defer func() {
		r.c <- roomQueueEntry{leavingClient: client}
	}()

	n := len(clients)
	if n == 1 {
		r.clients.Store(&noClients)
		return
	}

	f := make(Clients, n-1)
	copy(f, clients[:n-1])
	if idx != n-1 {
		f[idx] = clients[n-1]
	}
	r.clients.Store(&f)
}
