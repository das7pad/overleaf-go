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
	"github.com/golang-jwt/jwt/v4"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/jwt/epochJWT"
	"github.com/das7pad/overleaf-go/pkg/jwt/jwtHandler"
)

const (
	jwtCacheKey = "httpUtils.jwtClaims"
)

type PopulateClaims interface {
	Populate(c *gin.Context)
}

func NewJWTHandlerFromQuery(handler jwtHandler.JWTHandler, fromQuery string) *JWTHTTPHandler {
	return &JWTHTTPHandler{
		parser:    handler,
		fromQuery: fromQuery,
	}
}

func NewJWTHandler(handler jwtHandler.JWTHandler) *JWTHTTPHandler {
	return &JWTHTTPHandler{
		parser:    handler,
		fromQuery: "",
	}
}

type JWTHTTPHandler struct {
	parser    jwtHandler.JWTHandler
	fromQuery string
}

func (h *JWTHTTPHandler) Parse(c *gin.Context) (jwt.Claims, error) {
	var blob string
	if h.fromQuery != "" {
		blob = c.Query(h.fromQuery)
	} else {
		v := c.GetHeader("Authorization")
		if len(v) > 7 && v[:7] == "Bearer " {
			blob = v[7:]
		}
	}

	if blob == "" {
		return nil, &errors.UnauthorizedError{Reason: "missing jwt"}
	}

	claims, err := h.parser.Parse(blob)
	if err != nil {
		return nil, &errors.UnauthorizedError{Reason: "invalid jwt"}
	}

	if epochClaims, ok := claims.(epochJWT.EpochClaims); ok {
		if err = epochClaims.EpochItems().Check(c); err != nil {
			return nil, err
		}
	}

	c.Set(jwtCacheKey, claims)
	if populateClaims, ok := claims.(PopulateClaims); ok {
		populateClaims.Populate(c)
	}
	return claims, nil
}

func (h *JWTHTTPHandler) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		_, err := h.Parse(c)
		if err != nil {
			RespondErr(c, err)
			return
		}
	}
}