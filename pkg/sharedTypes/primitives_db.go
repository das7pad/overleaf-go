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
	"database/sql/driver"
	"encoding/binary"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes/internal/pgarray"
)

func (UUID) SkipUnderlyingTypePlan() {}

func (UUIDs) SkipUnderlyingTypePlan() {}

var (
	_ pgtype.SkipUnderlyingTypePlanner = UUID{}
	_ pgtype.Codec                     = UUIDCodec{}
	_ pgtype.SkipUnderlyingTypePlanner = UUIDs{}
	_ pgtype.Codec                     = UUIDsCodec{}
)

type UUIDCodec struct{}

func (UUIDCodec) FormatSupported(format int16) bool {
	return format == pgtype.BinaryFormatCode
}

func (UUIDCodec) PreferredFormat() int16 {
	return pgtype.BinaryFormatCode
}

func (c UUIDCodec) PlanEncode(_ *pgtype.Map, _ uint32, _ int16, _ any) pgtype.EncodePlan {
	return c
}

func (UUIDCodec) Encode(value any, buf []byte) ([]byte, error) {
	uuid := value.(UUID)
	return append(buf, uuid[:]...), nil
}

func (c UUIDCodec) PlanScan(_ *pgtype.Map, _ uint32, _ int16, _ any) pgtype.ScanPlan {
	return c
}

func (UUIDCodec) Scan(src []byte, dst any) error {
	uuid := dst.(*UUID)
	copy(uuid[:], src)
	return nil
}

func (c UUIDCodec) DecodeDatabaseSQLValue(_ *pgtype.Map, _ uint32, _ int16, _ []byte) (driver.Value, error) {
	return nil, errors.New("UUIDCodec.DecodeDatabaseSQLValue not implemented")
}

func (c UUIDCodec) DecodeValue(_ *pgtype.Map, _ uint32, _ int16, _ []byte) (any, error) {
	return nil, errors.New("UUIDCodec.DecodeValue not implemented")
}

type UUIDsCodec struct{}

func (UUIDsCodec) FormatSupported(format int16) bool {
	return format == pgtype.BinaryFormatCode
}

func (UUIDsCodec) PreferredFormat() int16 {
	return pgtype.BinaryFormatCode
}

func (c UUIDsCodec) PlanEncode(_ *pgtype.Map, _ uint32, _ int16, _ any) pgtype.EncodePlan {
	return c
}

func (UUIDsCodec) Encode(value any, buf []byte) ([]byte, error) {
	s := value.(UUIDs)

	buf = pgarray.EncodeFlatArrayHeader(buf, len(s), 16)

	for _, u := range s {
		// UUID length
		buf = binary.BigEndian.AppendUint32(buf, 16)
		// UUID body
		buf = append(buf, u[:]...)
	}
	return buf, nil
}

func (c UUIDsCodec) PlanScan(_ *pgtype.Map, _ uint32, _ int16, _ any) pgtype.ScanPlan {
	return c
}

func (UUIDsCodec) Scan(src []byte, dst any) error {
	s := dst.(*UUIDs)
	if len(src) == 0 {
		*s = (*s)[:0]
		return nil
	}
	src, n, err := pgarray.DecodeFlatArrayHeader(src, 16)
	if err != nil {
		return errors.Tag(err, "decode UUID array header")
	}
	s2 := *s
	if cap(s2) < n {
		s2 = make(UUIDs, n)
	} else {
		s2 = s2[:n]
	}
	offset := 0
	for i := 0; i < n; i++ {
		offset += 4 // UUID length, skip over [0, 0, 0, 16]
		copy(s2[i][:], src[offset:])
		offset += 16
	}
	*s = s2
	return nil
}

func (c UUIDsCodec) DecodeDatabaseSQLValue(_ *pgtype.Map, _ uint32, _ int16, _ []byte) (driver.Value, error) {
	return nil, errors.New("UUIDsCodec.DecodeDatabaseSQLValue not implemented")
}

func (c UUIDsCodec) DecodeValue(_ *pgtype.Map, _ uint32, _ int16, _ []byte) (any, error) {
	return nil, errors.New("UUIDsCodec.DecodeValue not implemented")
}
