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
	"encoding/json"

	"github.com/jackc/pgx/v5/pgtype"
)

func (o *Op) DecodeBinary(_ *pgtype.Map, src []byte) error {
	return json.Unmarshal(src, o)
}

func (o Op) EncodeBinary(_ *pgtype.Map, buf []byte) ([]byte, error) {
	b, err := json.Marshal(o)
	return append(buf, b...), err
}
