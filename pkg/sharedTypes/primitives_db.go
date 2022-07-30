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

//go:build !noDB

package sharedTypes

import (
	"fmt"

	"github.com/jackc/pgtype"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

func (u *UUID) DecodeBinary(_ *pgtype.ConnInfo, src []byte) error {
	copy(u[:], src)
	return nil
}

func (u *UUID) EncodeBinary(_ *pgtype.ConnInfo, buf []byte) (newBuf []byte, err error) {
	return append(buf, u[:]...), nil
}

func (s *UUIDs) DecodeBinary(ci *pgtype.ConnInfo, src []byte) error {
	if len(src) == 0 {
		*s = (*s)[:0]
		return nil
	}
	h := pgtype.ArrayHeader{}
	offset, err := h.DecodeBinary(ci, src)
	if err != nil {
		return errors.Tag(err, "decode UUID array header")
	}
	n := 0
	for _, dimension := range h.Dimensions {
		n = int(dimension.Length)
	}
	min := n * (4 + 16)
	if len(src[offset:]) < min {
		return errors.New(fmt.Sprintf(
			"decode UUID array header: too short body: expected >=%d, got %d",
			min, len(src[offset:]),
		))
	}
	s2 := *s
	if cap(s2) < n {
		s2 = make(UUIDs, n)
	} else {
		s2 = s2[:n]
	}
	for i := 0; i < n; i++ {
		offset += 4 // UUID length, skip over [0, 0, 0, 16]
		copy(s2[i][:], src[offset:])
		offset += 16
	}
	*s = s2
	return nil
}

func (s UUIDs) EncodeBinary(ci *pgtype.ConnInfo, buf []byte) (newBuf []byte, err error) {
	h := pgtype.ArrayHeader{
		ContainsNull: false,
		ElementOID:   pgtype.UUIDOID,
		Dimensions:   []pgtype.ArrayDimension{{Length: int32(len(s))}},
	}
	buf = h.EncodeBinary(ci, buf)
	min := len(buf) + len(s)*(16+4)
	if cap(buf) < min {
		newBuf = make([]byte, len(buf), min)
		copy(newBuf, buf)
		buf = newBuf
	}
	for _, u := range s {
		buf = append(buf, 0, 0, 0, 16) // UUID length
		buf = append(buf, u[:]...)
	}
	return buf, nil
}
