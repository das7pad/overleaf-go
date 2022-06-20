// Golang port of Overleaf
// Copyright (C) 2022 Jakob Ackermann <das7pad@outlook.com>
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

package m2pq

import (
	"encoding/hex"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

var (
	allZeroObjectIDString = "000000000000000000000000"
	allZeroObjectIDByte   = [12]byte{}
	ErrInvalidID          = &errors.ValidationError{Msg: "invalid id"}
)

func ObjectID2UUID(id [12]byte) sharedTypes.UUID {
	if id == allZeroObjectIDByte {
		return sharedTypes.UUID{}
	}
	return objectID2UUID(id[:])
}

func UUID2ObjectID(u sharedTypes.UUID) ([12]byte, error) {
	if u[6] != 64 || u[7] != 0 || u[8] != 128 || u[9] != 0 {
		return [12]byte{}, &errors.ValidationError{Msg: "not a mongo id"}
	}
	id := [12]byte{}
	copy(id[:6], u[:6])
	copy(id[6:], u[10:])
	return id, nil
}

func objectID2UUID(id []byte) sharedTypes.UUID {
	var u sharedTypes.UUID
	copy(u[:6], id[:6])
	copy(u[10:], id[6:])

	// Reset bits and populate version (4) and variant (10).
	u[6] = (u[6] & 0x0f) | 0x40
	u[8] = (u[8] & 0x3f) | 0x80
	return u
}

func ParseID(s string) (sharedTypes.UUID, error) {
	switch len(s) {
	case 24:
		if s == allZeroObjectIDString {
			return sharedTypes.UUID{}, nil
		}
		b, err := hex.DecodeString(s)
		if err != nil {
			return sharedTypes.UUID{}, ErrInvalidID
		}
		return objectID2UUID(b), nil
	case 36:
		return sharedTypes.ParseUUID(s)
	default:
		return sharedTypes.UUID{}, ErrInvalidID
	}
}
