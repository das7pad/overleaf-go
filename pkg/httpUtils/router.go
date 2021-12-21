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
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/das7pad/overleaf-go/pkg/httpTiming"
)

type RouterOptions struct {
	StatusMessage   string
	ClientIPOptions *ClientIPOptions
}

func NewRouter(options *RouterOptions) *gin.Engine {
	router := gin.New()
	if options.ClientIPOptions != nil {
		router.RemoteIPHeaders = []string{"X-Forwarded-For"}
		router.TrustedProxies = options.ClientIPOptions.TrustedProxies
	}
	router.Use(gin.Recovery())

	status := func(c *gin.Context) {
		c.String(http.StatusOK, options.StatusMessage)
	}
	router.GET("/status", status)
	router.HEAD("/status", status)

	router.Use(httpTiming.StartTotalTimer)
	return router
}
