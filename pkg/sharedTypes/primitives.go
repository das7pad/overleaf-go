// Golang port of Overleaf
// Copyright (C) 2021-2024 Jakob Ackermann <das7pad@outlook.com>
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
	"encoding/hex"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

var ErrInvalidUUID = &errors.ValidationError{Msg: "invalid uuid"}

func ParseUUID(s string) (UUID, error) {
	u := UUID{}
	return u, u.UnmarshalText([]byte(s))
}

func PopulateUUID(u *UUID) error {
	if _, err := rand.Read(u[:]); err != nil {
		return errors.Tag(err, "generate new UUID")
	}

	// Reset bits and populate version (4) and variant (10).
	u[6] = (u[6] & 0x0f) | 0x40
	u[8] = (u[8] & 0x3f) | 0x80
	return nil
}

func NewUUIDBatch(n int) UUIDBatch {
	return UUIDBatch{buf: make([]byte, 0, n*16)}
}

type UUIDBatch struct {
	buf    []byte
	offset int
}

func (b *UUIDBatch) Cap() int {
	return cap(b.buf) / 16
}

func (b *UUIDBatch) Len() int {
	return len(b.buf) / 16
}

func (b *UUIDBatch) Add(id UUID) {
	b.buf = append(b.buf, id[:]...)
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
		return nil, errors.Tag(err, "generate new UUIDs")
	}

	for i := 0; i < n; i++ {
		// Reset bits and populate version (4) and variant (10).
		buf[i*16+6] = (buf[i*16+6] & 0x0f) | 0x40
		buf[i*16+8] = (buf[i*16+8] & 0x3f) | 0x80
	}
	return &UUIDBatch{buf: buf}, nil
}

const AllZeroUUID = "00000000-0000-0000-0000-000000000000"

type UUID [16]byte

func (u UUID) IsZero() bool {
	return u == UUID{}
}

func (u UUID) readHexDecoded(dst []byte) {
	hex.Encode(dst, u[:4])
	dst[8] = '-'
	hex.Encode(dst[9:13], u[4:6])
	dst[13] = '-'
	hex.Encode(dst[14:18], u[6:8])
	dst[18] = '-'
	hex.Encode(dst[19:23], u[8:10])
	dst[23] = '-'
	hex.Encode(dst[24:], u[10:])
}

func (u UUID) String() string {
	dst := make([]byte, 36)
	u.readHexDecoded(dst)
	return string(dst)
}

func (u UUID) Append(b []byte) []byte {
	end := len(b) + 36
	if cap(b) < end {
		b = append(make([]byte, end), b...)
	}
	u.readHexDecoded(b[end-36 : end])
	return b[:end]
}

func (u UUID) Concat(c byte, other UUID) string {
	b := make([]byte, 0, 36+1+36)
	b = u.Append(b)
	b = append(b, c)
	b = other.Append(b)
	return string(b)
}

func (u UUID) MarshalJSON() ([]byte, error) {
	dst := make([]byte, 38)
	dst[0] = '"'
	dst[37] = '"'
	u.readHexDecoded(dst[1:37])
	return dst, nil
}

func (u *UUID) UnmarshalText(src []byte) error {
	if len(src) != 36 ||
		src[8] != '-' || src[13] != '-' || src[18] != '-' || src[23] != '-' {
		return ErrInvalidUUID
	}
	stripped := make([]byte, 32)
	copy(stripped[:8], src[:8])
	copy(stripped[8:12], src[9:13])
	copy(stripped[12:16], src[14:18])
	copy(stripped[16:20], src[19:23])
	copy(stripped[20:32], src[24:])
	if _, err := hex.Decode(u[:], stripped); err != nil {
		return ErrInvalidUUID
	}
	return nil
}

func (u *UUID) UnmarshalJSON(b []byte) error {
	if len(b) != 38 || b[0] != '"' || b[37] != '"' {
		return ErrInvalidUUID
	}
	return u.UnmarshalText(b[1:37])
}

type UUIDs []UUID
