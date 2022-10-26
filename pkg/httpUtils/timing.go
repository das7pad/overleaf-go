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

package httpUtils

import (
	"strconv"
	"time"
)

func TimeStage(c *Context, label string) func() {
	t0 := time.Now()
	return func() {
		endTimer(c, label, t0)
	}
}

func EndTotalTimer(c *Context) {
	endTimer(c, "total", c.t0)
}

func endTimer(c *Context, label string, t0 time.Time) {
	diff := time.Since(t0)
	ms := int64(diff / time.Millisecond)
	micro := int64(diff % time.Millisecond / time.Microsecond)
	c.Writer.Header().Add(
		"Server-Timing",
		// Inline printing of "%s;dur=%d.%03d" % (label, ms, micro)
		label+";dur="+
			strconv.FormatInt(ms, 10)+"."+
			strconv.FormatInt(micro%1000/100, 10)+
			strconv.FormatInt(micro%100/10, 10)+
			strconv.FormatInt(micro%10/1, 10),
	)
}
