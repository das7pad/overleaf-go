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

package httpUtils

import (
	"strings"

	"github.com/golang-jwt/jwt/v4"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/jwt/jwtHandler"
)

func init() {
	jwt.DecodeStrict = true
}

type JWT interface {
	jwtHandler.JWT
	PostProcess(c *Context) (*Context, error)
}

func NewJWTHandler[T JWT](handler jwtHandler.JWTHandler[T]) *JWTHTTPHandler[T] {
	return &JWTHTTPHandler[T]{
		parser: handler,
	}
}

type JWTHTTPHandler[T JWT] struct {
	parser jwtHandler.JWTHandler[T]
}

func (h *JWTHTTPHandler[T]) Parse(c *Context) (*Context, error) {
	var blob string
	v := c.Request.Header.Get("Authorization")
	if strings.HasPrefix(v, "Bearer ") {
		blob = v[7:]
	}
	if blob == "" {
		return c, &errors.UnauthorizedError{Reason: "missing jwt"}
	}

	claims, err := h.parser.Parse(blob)
	if err != nil {
		return c, &errors.UnauthorizedError{Reason: "invalid jwt"}
	}

	if c, err = claims.PostProcess(c); err != nil {
		return c, err
	}

	return c, nil
}

func (h *JWTHTTPHandler[T]) Middleware() MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(c *Context) {
			done := TimeStage(c, "jwt")
			c, err := h.Parse(c)
			done()
			if err != nil {
				RespondErr(c, err)
				return
			}
			next(c)
		}
	}
}
