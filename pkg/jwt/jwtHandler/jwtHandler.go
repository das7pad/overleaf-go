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

package jwtHandler

import (
	"time"

	"github.com/golang-jwt/jwt/v4"

	"github.com/das7pad/overleaf-go/pkg/options/jwtOptions"
)

type JWTHandler interface {
	New() jwt.Claims
	ExpiresIn() time.Duration
	Parse(blob string) (jwt.Claims, error)
	Sign(claims jwt.Claims) (string, error)
}

type NewClaims func() jwt.Claims

func New(options jwtOptions.JWTOptions, newClaims NewClaims) JWTHandler {
	method := jwt.GetSigningMethod(options.Algorithm)
	key := options.Key

	p := jwt.Parser{
		ValidMethods: []string{method.Alg()},
	}
	var keyFn jwt.Keyfunc = func(_ *jwt.Token) (interface{}, error) {
		return key, nil
	}
	return &handler{
		expiresIn: options.ExpiresIn,
		newClaims: newClaims,
		key:       key,
		keyFn:     keyFn,
		method:    method,
		p:         p,
	}
}

type handler struct {
	expiresIn time.Duration
	key       interface{}
	keyFn     jwt.Keyfunc
	method    jwt.SigningMethod
	newClaims NewClaims
	p         jwt.Parser
}

func (h *handler) ExpiresIn() time.Duration {
	return h.expiresIn
}

func (h *handler) New() jwt.Claims {
	return h.newClaims()
}

func (h *handler) Parse(blob string) (jwt.Claims, error) {
	t, err := h.p.ParseWithClaims(blob, h.newClaims(), h.keyFn)
	if err != nil {
		return nil, err
	}
	return t.Claims, nil
}

func (h *handler) Sign(claims jwt.Claims) (string, error) {
	t := jwt.NewWithClaims(h.method, claims)
	return t.SignedString(h.key)
}
