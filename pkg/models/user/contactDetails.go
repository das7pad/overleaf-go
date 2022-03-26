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

package user

import (
	"github.com/edgedb/edgedb-go"
)

type Contact struct {
	WithPublicInfoAndNonStandardId `edgedb:"$inline"`
	Connections                    edgedb.OptionalInt64    `edgedb:"connections"`
	LastTouched                    edgedb.OptionalDateTime `edgedb:"last_touched"`
}

func (a Contact) IsPreferredOver(b Contact) bool {
	aConnections, _ := a.Connections.Get()
	bConnections, _ := b.Connections.Get()
	if aConnections > bConnections {
		return true
	} else if aConnections < bConnections {
		return false
	}
	aLastTouched, _ := a.LastTouched.Get()
	bLastTouched, _ := b.LastTouched.Get()
	if aLastTouched.After(bLastTouched) {
		return true
	} else if aLastTouched.Before(bLastTouched) {
		return false
	} else {
		return false
	}
}
