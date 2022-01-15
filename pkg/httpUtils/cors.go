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
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type CORSOptions struct {
	AllowOrigins    []string
	AllowWebsockets bool
}

func (o *CORSOptions) originValid(origin string) bool {
	for _, allowedOrigin := range o.AllowOrigins {
		if origin == allowedOrigin {
			return true
		}
	}
	return false
}

func CORS(options CORSOptions) MiddlewareFunc {
	methods := strings.Join([]string{
		http.MethodDelete,
		http.MethodGet,
		http.MethodHead,
		http.MethodPost,
		http.MethodPut,
	}, ",")
	maxAge := strconv.FormatInt(int64(time.Hour.Seconds()), 10)
	return func(next HandlerFunc) HandlerFunc {
		return func(c *Context) {
			origin := c.Request.Header.Get("Origin")
			if origin == "" || origin == "https://"+c.Request.Host {
				// not a cross-origin request.
				next(c)
				return
			}
			h := c.Writer.Header()
			h.Set("Access-Control-Max-Age", maxAge)
			h.Set("Vary", "Origin")
			fmt.Println(c.Request.Header)
			fmt.Println(origin, options.AllowOrigins, c.Request.Host)
			if !options.originValid(origin) {
				c.Writer.WriteHeader(http.StatusForbidden)
				return
			}
			h.Set("Access-Control-Allow-Credentials", "true")
			h.Set("Access-Control-Allow-Headers", "Authorization")
			h.Set("Access-Control-Allow-Methods", methods)
			h.Set("Access-Control-Allow-Origin", origin)
			if c.Request.Method == http.MethodOptions {
				c.Writer.WriteHeader(http.StatusNoContent)
				return
			}
			next(c)
		}
	}
}
