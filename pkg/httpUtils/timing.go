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

package httpUtils

import (
	"github.com/gin-gonic/gin"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

const timingKeyTotal = "httpUtils.timing.total"

var (
	StartTotalTimer = StartTimer(timingKeyTotal)
	EndTotalTimer   = EndTimer(timingKeyTotal, "total")
)

func StartTimer(key string) gin.HandlerFunc {
	return func(c *gin.Context) {
		t := &sharedTypes.Timed{}
		t.Begin()
		c.Set(key, t)
	}
}

func EndTimer(key string, label string) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, exists := c.Get(key)
		if !exists {
			return
		}
		t := raw.(*sharedTypes.Timed)
		t.End()
		c.Writer.Header().Add("Server-Timing", label+";dur="+t.MS())
	}
}
