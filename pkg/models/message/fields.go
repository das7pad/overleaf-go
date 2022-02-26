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

package message

import (
	"time"

	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/models/user"
)

type IdField struct {
	Id edgedb.UUID `edgedb:"id" json:"id"`
}

type ContentField struct {
	Content string `edgedb:"content" json:"content"`
}

type CreatedAtField struct {
	CreatedAt time.Time `edgedb:"created_at" json:"timestamp"`
}

type UserField struct {
	User user.WithPublicInfo `edgedb:"user"`
}

type EditedAtField struct {
	EditedAt edgedb.OptionalDateTime `edgedb:"edited_at" json:"edited_at"`
}

type RoomIdField struct {
	RoomId edgedb.UUID `edgedb:"room_id"`
}
