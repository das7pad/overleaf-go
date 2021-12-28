// Golang port of Overleaf
// Copyright (C) 2021 Jakob Ackermann <das7pad@outlook.com>
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

package types

import (
	"context"
	"time"
)

type SmokeTestStep struct {
	Name     string                          `json:"name"`
	Duration string                          `json:"duration"`
	Action   func(ctx context.Context) error `json:"-"`
}

type SmokeTestStats struct {
	Start    time.Time        `json:"start"`
	Steps    []*SmokeTestStep `json:"steps"`
	End      time.Time        `json:"end"`
	Duration string           `json:"duration"`
}

type SmokeTestResponse struct {
	Stats *SmokeTestStats `json:"stats"`
}
