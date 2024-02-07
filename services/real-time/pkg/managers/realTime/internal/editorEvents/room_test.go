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
	"testing"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

func clientsEqual(a, b types.Clients) bool {
	if len(a) != len(b) {
		return false
	}
	for i, client := range a {
		if b[i] != client {
			return false
		}
	}
	return true
}

func Test_room_remove(t *testing.T) {
	a := &types.Client{PublicId: "a"}
	b := &types.Client{PublicId: "b"}
	c := &types.Client{PublicId: "c"}
	d := &types.Client{PublicId: "d"}
	e := &types.Client{PublicId: "e"}
	f := &types.Client{PublicId: "f"}
	all := types.Clients{a, b, c, d, e, f}
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
					for _, c4 := range all {
						if c4 == c0 || c4 == c1 || c4 == c2 || c4 == c3 {
							continue
						}
						permutations = append(permutations, types.Clients{
							c0, c1, c2, c3, c4,
						})
						for _, c5 := range all {
							if c5 == c0 || c5 == c1 || c5 == c2 || c5 == c3 || c5 == c4 {
								continue
							}
							permutations = append(permutations, types.Clients{
								c0, c1, c2, c3, c4, c5,
							})
						}
					}
				}
			}
		}
	}
	fRc := func(sharedTypes.UUID, RoomChanges) {}
	fP := func(context.Context, sharedTypes.UUID) bool { return true }
	rc := make(RoomChanges, 0, len(all)*2)
	r := newRoom(sharedTypes.UUID{}, fRc, fP)
	close(r.c)
	r.roomChangesFlush.Stop()

	for i1, p1 := range permutations {
		for i2, p2 := range permutations {
			r.clients.Store(noClients)
			<-r.roomChanges
			r.roomChanges <- rc[:0]
			for _, client := range p1 {
				r.add(client)
			}
			if got := r.Clients().All; !clientsEqual(got, p1) {
				t.Fatalf("%d/%d add=%s != clients=%s", i1, i2, p1, got)
			}
			for i3, client := range p2 {
				if p1.Index(client) != -1 {
					r.remove(client)
				}
				clients := r.Clients()
				for i, other := range clients.All {
					if clients.Removed.Has(i) {
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
		}
	}
	close(r.roomChanges)
	<-r.roomChanges
}

func TestRemovedClients_Add(t *testing.T) {
	type want struct {
		r  RemovedClients
		ok bool
	}
	tests := []struct {
		name string
		r    RemovedClients
		i    int
		want want
	}{
		{
			name: "first",
			r:    RemovedClients{-1, -1, -1, -1},
			i:    0,
			want: want{
				r:  RemovedClients{0, -1, -1, -1},
				ok: true,
			},
		},
		{
			name: "2nd before",
			r:    RemovedClients{1, -1, -1, -1},
			i:    0,
			want: want{
				r:  RemovedClients{0, 1, -1, -1},
				ok: true,
			},
		},
		{
			name: "2nd after",
			r:    RemovedClients{0, -1, -1, -1},
			i:    1,
			want: want{
				r:  RemovedClients{0, 1, -1, -1},
				ok: true,
			},
		},
		{
			name: "3rd before",
			r:    RemovedClients{1, 2, -1, -1},
			i:    0,
			want: want{
				r:  RemovedClients{0, 1, 2, -1},
				ok: true,
			},
		},
		{
			name: "3rd middle",
			r:    RemovedClients{0, 2, -1, -1},
			i:    1,
			want: want{
				r:  RemovedClients{0, 1, 2, -1},
				ok: true,
			},
		},
		{
			name: "3rd after",
			r:    RemovedClients{0, 1, -1, -1},
			i:    2,
			want: want{
				r:  RemovedClients{0, 1, 2, -1},
				ok: true,
			},
		},
		{
			name: "4th 0",
			r:    RemovedClients{1, 2, 3, -1},
			i:    0,
			want: want{
				r:  RemovedClients{0, 1, 2, 3},
				ok: true,
			},
		},
		{
			name: "4th 1",
			r:    RemovedClients{0, 2, 3, -1},
			i:    1,
			want: want{
				r:  RemovedClients{0, 1, 2, 3},
				ok: true,
			},
		},
		{
			name: "4th 2",
			r:    RemovedClients{0, 1, 3, -1},
			i:    2,
			want: want{
				r:  RemovedClients{0, 1, 2, 3},
				ok: true,
			},
		},
		{
			name: "4th 3",
			r:    RemovedClients{0, 1, 2, -1},
			i:    3,
			want: want{
				r:  RemovedClients{0, 1, 2, 3},
				ok: true,
			},
		},
		{
			name: "5th",
			r:    RemovedClients{0, 1, 2, 3},
			i:    4,
			want: want{
				r:  RemovedClients{0, 1, 2, 3},
				ok: false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, got := tt.r.Add(tt.i)
			if got != tt.want.ok {
				t.Errorf("Add() = %v, want %v", got, tt.want.ok)
			}
			if r != tt.want.r {
				t.Errorf("Add() -> %v, want %v", r, tt.want.r)
			}
		})
	}
}
