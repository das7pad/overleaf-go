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
	"github.com/das7pad/real-time/pkg/managers/realTime/internal/pendingOperation"
	"github.com/das7pad/real-time/pkg/types"
)

type flatClients []*types.Client

var noClients = make(flatClients, 0)

type room struct {
	flat flatClients

	pendingSubscribe   pendingOperation.WithCancel
	pendingUnsubscribe pendingOperation.WithCancel
}

func (r *room) isEmpty() bool {
	return len(r.flat) == 0
}

func (r *room) add(client *types.Client) {
	if r.isEmpty() {
		r.flat = flatClients{client}
		return
	}

	for _, c := range r.flat {
		if c == client {
			return
		}
	}

	n := len(r.flat) + 1
	f := make(flatClients, n)
	copy(f, r.flat)
	f[n-1] = client
	r.flat = f
}

func (r *room) remove(client *types.Client) {
	if r.isEmpty() {
		return
	}
	idx := -1
	for i, c := range r.flat {
		if c == client {
			idx = i
			break
		}
	}
	if idx == -1 {
		// Not found.
		return
	}

	n := len(r.flat)
	if n == 1 {
		r.flat = noClients
		return
	}

	f := make(flatClients, n-1)
	copy(f, r.flat[:n-1])
	if idx != n-1 {
		f[n-2] = r.flat[n-1]
	}
	r.flat = f
}
