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

package pgarray

import (
	"encoding/binary"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

func EncodeFlatArrayHeader(buf []byte, length, elementSize int) []byte {
	n := len(buf) + 5*4 + length*(elementSize+4)
	if cap(buf) < n {
		buf = append(make([]byte, 0, n), buf...)
	}

	// dimensions
	buf = binary.BigEndian.AppendUint32(buf, 1)

	// contains null
	buf = binary.BigEndian.AppendUint32(buf, 0)

	// element oid
	buf = binary.BigEndian.AppendUint32(buf, pgtype.UUIDOID)

	// dimension 1, length
	buf = binary.BigEndian.AppendUint32(buf, uint32(length))
	// dimension 1, lower bound
	buf = binary.BigEndian.AppendUint32(buf, 1)

	return buf
}

func DecodeFlatArrayHeader(src []byte, elementSize int) ([]byte, int, error) {
	if len(src) < 12 {
		return nil, 0, errors.New(fmt.Sprintf(
			"too short src: expected >=%d, got %d",
			12, len(src),
		))
	}
	offset := 0

	dimensions := int(binary.BigEndian.Uint32(src))
	offset += 4
	if dimensions > 1 {
		return nil, 0, errors.New(fmt.Sprintf(
			"too many dimensions: expected <=1, got %d",
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
	expected := n * (4 + elementSize)
	if got := len(src[offset:]); got < expected {
		return nil, 0, errors.New(fmt.Sprintf(
			"too short body: expected >=%d, got %d",
			expected, got,
		))
	}
	return src[offset:], n, nil
}
