// Golang port of Overleaf
// Copyright (C) 2022-2023 Jakob Ackermann <das7pad@outlook.com>
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
	"encoding/binary"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

func (UUID) SkipUnderlyingTypePlan() {}

func (u *UUID) ScanUUID(v pgtype.UUID) error {
	copy(u[:], v.Bytes[:])
	return nil
}

func (u *UUID) UUIDValue() (pgtype.UUID, error) {
	return pgtype.UUID{Bytes: *u, Valid: true}, nil
}

func (s *UUIDs) DecodeBinary(ci *pgtype.Map, src []byte) error {
	if len(src) == 0 {
		*s = (*s)[:0]
		return nil
	}
	if len(src) < 12 {
		return errors.New(fmt.Sprintf(
			"decode UUID array header: too short body: expected >=%d, got %d",
			12, len(src),
		))
	}
	offset := 0

	dimensions := int(binary.BigEndian.Uint32(src))
	offset += 4
	if dimensions > 1 {
		return errors.New(fmt.Sprintf(
			"decode UUID array header: too many dimensions: expected <=1, got %d",
			dimensions,
		))
	}

	// contains null
	offset += 4

	// OID
	offset += 4

	// first dimension
	n := 0
	if len(src) >= 12+4 {
		// length
		n = int(binary.BigEndian.Uint32(src[offset:]))
		offset += 4

		// lower bound
		offset += 4
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

func (s UUIDs) EncodeBinary(ci *pgtype.Map, buf []byte) ([]byte, error) {
	min := len(buf) + 5*4 + len(s)*(16+4)
	if cap(buf) < min {
		buf = append(make([]byte, 0, min), buf...)
	}
	// dimensions
	buf = binary.BigEndian.AppendUint32(buf, 1)

	// contains null
	buf = binary.BigEndian.AppendUint32(buf, 0)

	// element oid
	buf = binary.BigEndian.AppendUint32(buf, pgtype.UUIDOID)

	for _, u := range s {
		// UUID length
		buf = binary.BigEndian.AppendUint32(buf, 16)
		// UUID body
		buf = append(buf, u[:]...)
	}
	return buf, nil
}
