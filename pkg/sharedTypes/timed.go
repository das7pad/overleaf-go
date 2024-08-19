// Golang port of Overleaf
// Copyright (C) 2021-2024 Jakob Ackermann <das7pad@outlook.com>
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
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
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

func (t *Timed) SetDiff(diff time.Duration) {
	t.diff = diff
}

func (t *Timed) End() *Timed {
	if t.t0.IsZero() {
		return t
	}
	t.diff = time.Since(t.t0).Truncate(time.Microsecond)
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

func (t Timed) MarshalJSON() ([]byte, error) {
	s := t.diff.String()
	buf := make([]byte, 0, 1+len(s)+1)
	return append(append(append(buf, '"'), s...), '"'), nil
}

var errBadTimed = errors.New("bad duration")

func (t *Timed) UnmarshalJSON(bytes []byte) error {
	if len(bytes) < 3 || bytes[0] != '"' || bytes[len(bytes)-1] != '"' {
		return errBadTimed
	}
	var err error
	t.diff, err = time.ParseDuration(string(bytes[1 : len(bytes)-1]))
	return err
}
