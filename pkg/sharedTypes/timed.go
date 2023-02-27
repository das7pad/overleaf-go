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

package sharedTypes

import (
	"encoding/json"
	"time"
)

type Timed struct {
	t0   time.Time
	diff time.Duration
}

func (t *Timed) Begin() {
	t.t0 = time.Now()
}

func (t *Timed) SetBegin(t0 time.Time) {
	t.t0 = t0
}

func (t *Timed) End() *Timed {
	if t.t0.IsZero() {
		return t
	}
	t.diff = time.Since(t.t0)
	t.t0 = time.Time{}
	return t
}

func (t *Timed) String() string {
	return t.diff.String()
}

func (t *Timed) Stage() string {
	t.End()
	s := t.diff.String()
	t.Begin()
	return s
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
