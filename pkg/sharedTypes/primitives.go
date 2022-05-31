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

package sharedTypes

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

var ErrInvalidUUID = &errors.ValidationError{Msg: "invalid uuid"}

type Float float64

func (j Float) String() string {
	return strconv.FormatFloat(float64(j), 'f', -1, 64)
}

type Int int64

func (i Int) String() string {
	return strconv.FormatInt(int64(i), 10)
}

func ParseUUID(s string) (UUID, error) {
	if len(s) != 36 {
		return UUID{}, ErrInvalidUUID
	}
	u := UUID{}

	n := 0
	for i := 0; i < 16; i++ {
		x, err := strconv.ParseUint(s[n:n+2], 16, 8)
		if err != nil {
			return UUID{}, ErrInvalidUUID
		}
		u[i] = byte(x)
		n += 2
		if n == 8 || n == 13 || n == 18 || n == 23 {
			if s[n] != '-' {
				return UUID{}, ErrInvalidUUID
			}
			n++
		}
	}
	return u, nil
}

type UUID [16]byte

func (u UUID) String() string {
	return fmt.Sprintf(
		"%x-%x-%x-%x-%x",
		u[0:4],
		u[4:6],
		u[6:8],
		u[8:10],
		u[10:16],
	)
}

func (u *UUID) Scan(x any) error {
	b, ok := x.([]byte)
	if !ok {
		return errors.New(fmt.Sprintf("unexpected uuid src: %q", x))
	}
	u2, err := ParseUUID(string(b))
	if err != nil {
		return err
	}
	*u = u2
	return nil
}

func (u UUID) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.String())
}

func (u *UUID) UnmarshalJSON(b []byte) error {
	s := ""
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	u2, err := ParseUUID(s)
	if err != nil {
		return err
	}
	*u = u2
	return nil
}
