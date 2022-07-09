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
	clients Clients
	c       chan roomQueueEntry

	pending pendingOperation.WithCancel
}

func (r *TrackingRoom) Handle(_ string) {
	panic("not implemented")
}

func (r *TrackingRoom) Clients() Clients {
	return r.clients
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
	return len(r.clients) == 0
}

func (r *TrackingRoom) add(client *types.Client) {
	if r.isEmpty() {
		client.AddWriter()
		r.clients = Clients{client}
		return
	}

	for _, c := range r.clients {
		if c == client {
			return
		}
	}

	n := len(r.clients) + 1
	f := make(Clients, n)
	copy(f, r.clients)
	f[n-1] = client
	client.AddWriter()
	r.clients = f
}

func (r *TrackingRoom) remove(client *types.Client) {
	if r.isEmpty() {
		return
	}
	idx := -1
	for i, c := range r.clients {
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

	n := len(r.clients)
	if n == 1 {
		r.clients = noClients
		return
	}

	f := make(Clients, n-1)
	copy(f, r.clients[:n-1])
	if idx != n-1 {
		f[idx] = r.clients[n-1]
	}
	r.clients = f
}
