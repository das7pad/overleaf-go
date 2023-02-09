// Golang port of Overleaf
// Copyright (C) 2021-2023 Jakob Ackermann <das7pad@outlook.com>
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
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Features struct {
	Collaborators       int                        `json:"collaborators"`
	CompileTimeout      sharedTypes.ComputeTimeout `json:"compileTimeout"`
	CompileGroup        sharedTypes.CompileGroup   `json:"compileGroup"`
	TrackChanges        bool                       `json:"trackChanges"`
	TrackChangesVisible bool                       `json:"trackChangesVisible"`
	Versioning          bool                       `json:"versioning"`
}

// TODO
//
// func (f *Features) DecodeBinary(ci *pgtype.Map, src []byte) error {
// 	b := pgtype.JSONB{}
// 	if err := b.DecodeBinary(ci, src); err != nil {
// 		return errors.Tag(err, "deserialize Features")
// 	}
// 	return json.Unmarshal(b.Bytes, f)
// }
//
// func (f Features) EncodeBinary(ci *pgtype.Map, buf []byte) ([]byte, error) {
// 	blob, err := json.Marshal(map[string]interface{}{
// 		"compileGroup":   f.CompileGroup,
// 		"compileTimeout": f.CompileTimeout,
// 	})
// 	if err != nil {
// 		return nil, errors.Tag(err, "serialize Features")
// 	}
// 	return pgtype.JSONB{
// 		Bytes:  blob,
// 		Status: pgtype.Present,
// 	}.EncodeBinary(ci, buf)
// }
