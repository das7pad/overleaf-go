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

package sharedTypes

import (
	"encoding/json"
	"time"
)

type Timed struct {
	t0   *time.Time
	diff time.Duration
}

func (t *Timed) Begin() {
	now := time.Now()
	t.t0 = &now
}

func (t *Timed) SetBegin(t0 time.Time) {
	t.t0 = &t0
}

func (t *Timed) End() {
	if t.t0 == nil {
		return
	}
	t.diff = time.Now().Sub(*t.t0)
	t.t0 = nil
}

func (t *Timed) Diff() int64 {
	return t.diff.Milliseconds()
}

func (t *Timed) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.diff.String())
}

func (t *Timed) UnmarshalJSON(bytes []byte) error {
	var raw string
	err := json.Unmarshal(bytes, &raw)
	if err != nil {
		return err
	}
	t.diff, err = time.ParseDuration(raw)
	return err
}
