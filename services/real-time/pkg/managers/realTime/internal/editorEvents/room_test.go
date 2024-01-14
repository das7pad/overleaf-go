// Golang port of Overleaf
// Copyright (C) 2023-2024 Jakob Ackermann <das7pad@outlook.com>
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
	"reflect"
	"testing"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

func Test_room_remove(t *testing.T) {
	a := &types.Client{PublicId: "a"}
	b := &types.Client{PublicId: "b"}
	c := &types.Client{PublicId: "c"}
	d := &types.Client{PublicId: "d"}
	all := types.Clients{a, b, c, d}
	var permutations []types.Clients
	for _, c0 := range all {
		permutations = append(permutations, types.Clients{
			c0,
		})
		for _, c1 := range all {
			if c1 == c0 {
				continue
			}
			permutations = append(permutations, types.Clients{
				c0, c1,
			})
			for _, c2 := range all {
				if c2 == c0 || c2 == c1 {
					continue
				}
				permutations = append(permutations, types.Clients{
					c0, c1, c2,
				})
				for _, c3 := range all {
					if c3 == c0 || c3 == c1 || c3 == c2 {
						continue
					}
					permutations = append(permutations, types.Clients{
						c0, c1, c2, c3,
					})
				}
			}
		}
	}
	fRc := func(sharedTypes.UUID, RoomChanges) {}
	fP := func(context.Context, sharedTypes.UUID) bool { return true }

	for i1, p1 := range permutations {
		for i2, p2 := range permutations {
			r := newRoom(sharedTypes.UUID{}, fRc, fP)
			<-r.roomChanges
			r.roomChanges <- make(RoomChanges, 0, 100)
			close(r.c)
			r.c = make(chan roomQueueEntry)
			go func() {
				for range r.c {
				}
			}()
			for _, client := range p1 {
				r.add(client)
			}
			if got := r.Clients().All; !reflect.DeepEqual(got, p1) {
				t.Fatalf("%d/%d add=%s != clients=%s", i1, i2, p1, got)
			}
			for i3, client := range p2 {
				if p1.Index(client) != -1 {
					r.remove(client)
				}
				clients := r.Clients()
				for i, other := range clients.All {
					if i == clients.Removed {
						continue
					}
					if client == other {
						t.Fatalf("%d/%d/%d add=%s, clients=%s, not removed=%s", i1, i2, i3, p1, clients.All, client)
					}
				}
			}
			if got := r.Clients().All; i1 == i2 && len(got) != 0 {
				t.Fatalf("%d/%d add+remove=%s, clients=%s", i1, i2, p1, got)
			}
			close(r.roomChanges)
			<-r.roomChanges
			close(r.c)
		}
	}
}
