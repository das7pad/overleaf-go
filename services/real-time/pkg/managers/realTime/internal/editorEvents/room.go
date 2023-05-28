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
	"sync/atomic"

	"github.com/das7pad/overleaf-go/pkg/pendingOperation"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

func newRoom() *room {
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

type roomQueueEntry struct {
	msg           string
	leavingClient *types.Client
}

type Clients = []*types.Client

type room struct {
	clients atomic.Pointer[Clients]
	c       chan roomQueueEntry

	pending pendingOperation.WithCancel
}

var noClients = &Clients{}

func (r *room) Clients() Clients {
	return *r.clients.Load()
}

func (r *room) broadcast(msg string) {
	r.c <- roomQueueEntry{msg: msg}
}

func (r *room) close() {
	close(r.c)
}

func (r *room) isEmpty() bool {
	return r.clients.Load() == noClients
}

func (r *room) add(client *types.Client) bool {
	p := r.clients.Load()
	if p == noClients {
		r.clients.Store(&Clients{client})
		return true
	}
	clients := *p
	clients = append(clients, client)
	r.clients.Store(&clients)
	return false
}

func (r *room) remove(client *types.Client) bool {
	defer func() {
		r.c <- roomQueueEntry{leavingClient: client}
	}()

	p := r.clients.Load()
	if p == noClients {
		return true
	}
	clients := *p
	idx := -1
	for i, c := range clients {
		if c == client {
			idx = i
			break
		}
	}
	if idx == -1 {
		// Not found.
		return false
	}

	n := len(clients)
	if n == 1 {
		r.clients.Store(noClients)
		return true
	}

	f := make(Clients, n-1, n)
	copy(f, clients[:n-1])
	if idx != n-1 {
		f[idx] = clients[n-1]
	}
	r.clients.Store(&f)
	return false
}
