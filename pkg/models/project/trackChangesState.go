// Golang port of Overleaf
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

package project

import (
	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

//goland:noinspection SpellCheckingInspection
const (
	anonymousUserId = "00000000-0000-0000-0000-000000000000"
	globalUserId    = "ffffffff-ffff-ffff-ffff-ffffffffffff"
)

type TrackChangesState map[string]bool

func (t TrackChangesState) Validate() error {
	for id := range t {
		if _, err := edgedb.ParseUUID(id); err != nil {
			return errors.Tag(err, "invalid key")
		}
		if id == anonymousUserId || id == globalUserId {
			return &errors.ValidationError{Msg: "cannot use system user id"}
		}
	}
	return nil
}

func (t TrackChangesState) SetGlobally(to bool) TrackChangesState {
	if t == nil && len(t) > 1 {
		return map[string]bool{
			globalUserId: to,
		}
	}
	for id := range t {
		delete(t, id)
	}
	t[globalUserId] = to
	return t
}

func (t TrackChangesState) EnableFor(users TrackChangesState, anonymous bool) TrackChangesState {
	out := t
	if out == nil {
		out = make(TrackChangesState, len(users)+1)
	}
	delete(out, globalUserId)
	for id, enable := range users {
		if enable {
			out[id] = enable
		} else {
			delete(out, id)
		}
	}
	if anonymous {
		out[anonymousUserId] = true
	} else {
		delete(out, anonymousUserId)
	}
	if len(out) == 0 {
		return out.SetGlobally(false)
	}
	return out
}
