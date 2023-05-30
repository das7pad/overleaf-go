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
	"strconv"
	"sync/atomic"

	"github.com/das7pad/overleaf-go/pkg/pendingOperation"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/events"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

func newRoom() *room {
	c := make(chan roomQueueEntry, 20)
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
			if entry.gracefulReconnect != 0 {
				r.handleGracefulReconnect(entry.gracefulReconnect)
				continue
			}
			r.Handle(entry.msg)
		}
	}()
	return r
}

type roomQueueEntry struct {
	msg               string
	leavingClient     *types.Client
	gracefulReconnect uint8
}

type Clients struct {
	All     types.Clients
	Removed int
}

func (c Clients) String() string {
	s := "all=["
	for i, client := range c.All {
		if i > 0 {
			s += ", "
		}
		s += client.String()
	}
	s += "] removed=" + strconv.FormatInt(int64(c.Removed), 10)
	return s
}

type room struct {
	clients atomic.Pointer[Clients]
	c       chan roomQueueEntry

	pending pendingOperation.PendingOperation
}

var noClients = &Clients{Removed: -1}

func (r *room) Clients() Clients {
	return *r.clients.Load()
}

func (r *room) broadcast(msg string) {
	r.c <- roomQueueEntry{msg: msg}
}

func (r *room) queueLeavingClient(client *types.Client) {
	r.c <- roomQueueEntry{leavingClient: client}
}

func (r *room) broadcastGracefulReconnect(suffix uint8) {
	r.c <- roomQueueEntry{gracefulReconnect: suffix}
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
		r.clients.Store(&Clients{All: []*types.Client{client}, Removed: -1})
		return true
	}
	clients := *p
	clients.All = append(clients.All, client)
	r.clients.Store(&clients)
	return false
}

func (r *room) remove(client *types.Client) bool {
	defer r.queueLeavingClient(client)

	p := r.clients.Load()
	if p == noClients {
		return true
	}
	idx := p.All.Index(client)
	if idx == -1 {
		// Not found.
		return false
	}

	n := len(p.All)
	if n == 1 || (n == 2 && p.Removed != -1 && p.Removed != idx) {
		r.clients.Store(noClients)
		return true
	}

	clients := *p
	if p.Removed == -1 {
		clients.Removed = idx
	} else {
		f := make(types.Clients, n-2, n)
		copy(f, clients.All[:n-2])
		if idx < n-2 {
			if clients.Removed == n-1 {
				f[idx] = clients.All[n-2]
			} else {
				f[idx] = clients.All[n-1]
			}
		}
		if clients.Removed < n-2 {
			if idx == n-1 || idx < n-2 {
				f[clients.Removed] = clients.All[n-2]
			} else {
				f[clients.Removed] = clients.All[n-1]
			}
		}
		clients.All = f
		clients.Removed = -1
	}
	r.clients.Store(&clients)
	return false
}

func (r *room) handleGracefulReconnect(suffix uint8) {
	clients := r.Clients()
	for i, client := range clients.All {
		if i == clients.Removed {
			continue
		}
		// The last character of a PublicId is a random hex char.
		if client.PublicId[32] != suffix {
			continue
		}
		client.EnsureQueueMessage(events.ReconnectGracefullyPrepared)
	}
}
