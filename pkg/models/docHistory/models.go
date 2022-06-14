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
	"time"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type DocHistory struct {
	UserId  sharedTypes.UUID
	Version sharedTypes.Version
	StartAt time.Time
	EndAt   time.Time
	Op      sharedTypes.Op
}

type ProjectUpdate struct {
	DocId        sharedTypes.UUID
	UserId       sharedTypes.UUID
	Version      sharedTypes.Version
	HasBigDelete bool
	StartAt      time.Time
	EndAt        time.Time
}

type ForInsert struct {
	UserId       sharedTypes.UUID    `json:"user_id"`
	Version      sharedTypes.Version `json:"version"`
	HasBigDelete bool                `json:"has_big_delete"`
	StartAt      time.Time           `json:"start_at"`
	EndAt        time.Time           `json:"end_at"`
	Op           sharedTypes.Op      `json:"op"`
}
