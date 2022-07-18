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
	"crypto/rand"
	"database/sql/driver"
	"encoding/hex"
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
	if len(s) != 36 ||
		s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return UUID{}, ErrInvalidUUID
	}
	u := UUID{}
	src := make([]byte, 32)
	copy(src[:8], s[:8])
	copy(src[8:12], s[9:13])
	copy(src[12:16], s[14:18])
	copy(src[16:20], s[19:23])
	copy(src[20:32], s[24:])
	if _, err := hex.Decode(u[:], src); err != nil {
		return UUID{}, ErrInvalidUUID
	}
	return u, nil
}

func GenerateUUID() (UUID, error) {
	u := UUID{}
	return u, PopulateUUID(&u)
}

func PopulateUUID(u *UUID) error {
	if _, err := rand.Read(u[:]); err != nil {
		return errors.Tag(err, "cannot generate new UUID")
	}

	// Reset bits and populate version (4) and variant (10).
	u[6] = (u[6] & 0x0f) | 0x40
	u[8] = (u[8] & 0x3f) | 0x80
	return nil
}

type UUIDBatch struct {
	buf    []byte
	offset int
}

func (b *UUIDBatch) Next() UUID {
	u := UUID{}
	copy(u[:], b.buf[b.offset:b.offset+16])
	b.offset += 16
	return u
}

func GenerateUUIDBulk(n int) (*UUIDBatch, error) {
	buf := make([]byte, n*16)
	if _, err := rand.Read(buf); err != nil {
		return nil, errors.Tag(err, "cannot generate new UUIDs")
	}

	for i := 0; i < n; i++ {
		// Reset bits and populate version (4) and variant (10).
		buf[i*16+6] = (buf[i*16+6] & 0x0f) | 0x40
		buf[i*16+8] = (buf[i*16+8] & 0x3f) | 0x80
	}
	return &UUIDBatch{buf: buf}, nil
}

type UUID [16]byte

func (u UUID) String() string {
	dst := make([]byte, 36)
	hex.Encode(dst, u[:4])
	dst[8] = '-'
	hex.Encode(dst[9:13], u[4:6])
	dst[13] = '-'
	hex.Encode(dst[14:18], u[6:8])
	dst[18] = '-'
	hex.Encode(dst[19:23], u[8:10])
	dst[23] = '-'
	hex.Encode(dst[24:], u[10:])
	return string(dst)
}

func (u *UUID) Scan(x interface{}) error {
	var s string
	if b, ok := x.([]byte); ok {
		s = string(b)
	} else if s, ok = x.(string); !ok {
		return errors.New(fmt.Sprintf("unexpected uuid src: %q", x))
	}
	u2, err := ParseUUID(s)
	if err != nil {
		return err
	}
	*u = u2
	return nil
}

func (u UUID) Value() (driver.Value, error) {
	return u.String(), nil
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
