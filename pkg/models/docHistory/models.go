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

package docHistory

import (
	"encoding/json"
	"time"

	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type DocHistory struct {
	User    OptionalIdField     `edgedb:"user"`
	Version sharedTypes.Version `edgedb:"version"`
	StartAt time.Time           `edgedb:"start_at"`
	EndAt   time.Time           `edgedb:"end_at"`
	Op      sharedTypes.Op      `edgedb:"-"`
	OpForDB json.RawMessage     `edgedb:"op"`
}

type ProjectUpdate struct {
	Doc          IdField             `edgedb:"doc"`
	User         OptionalIdField     `edgedb:"user"`
	Version      sharedTypes.Version `edgedb:"version"`
	HasBigDelete bool                `edgedb:"has_big_delete"`
	StartAt      time.Time           `edgedb:"start_at"`
	EndAt        time.Time           `edgedb:"end_at"`
}

type ForInsert struct {
	UserId       edgedb.UUID         `json:"user_id"`
	Version      sharedTypes.Version `json:"version"`
	HasBigDelete bool                `json:"has_big_delete"`
	StartAt      time.Time           `json:"start_at"`
	EndAt        time.Time           `json:"end_at"`
	Op           sharedTypes.Op      `json:"-"`
	OpForDB      json.RawMessage     `json:"op"`
}
