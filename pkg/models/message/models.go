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

package message

import (
	"time"

	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/models/user"
)

type Message struct {
	Id        edgedb.UUID                         `json:"id" edgedb:"id"`
	Content   string                              `json:"content" edgedb:"content"`
	Timestamp time.Time                           `json:"timestamp" edgedb:"created_at"`
	User      user.WithPublicInfoAndNonStandardId `json:"user,omitempty" edgedb:"user"`
	EditedAt  edgedb.OptionalDateTime             `json:"edited_at,omitempty" edgedb:"edited_at"`
}

type Thread struct {
	Id             edgedb.UUID                         `json:"-" edgedb:"id"`
	Resolved       bool                                `json:"resolved,omitempty"`
	ResolvedAt     edgedb.OptionalDateTime             `json:"resolved_at,omitempty"`
	ResolvedByUser user.WithPublicInfoAndNonStandardId `json:"resolved_by,omitempty"`
	Messages       []Message                           `json:"messages"`
}

type Threads map[string]Thread