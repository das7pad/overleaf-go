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

package jwtHandler

import (
	"time"

	"github.com/golang-jwt/jwt/v4"

	"github.com/das7pad/overleaf-go/pkg/jwt/expiringJWT"
	"github.com/das7pad/overleaf-go/pkg/options/jwtOptions"
)

type JWTHandler interface {
	New() expiringJWT.ExpiringJWT
	Parse(blob string) (expiringJWT.ExpiringJWT, error)
	SetExpiryAndSign(claims expiringJWT.ExpiringJWT) (string, error)
}

type NewClaims func() expiringJWT.ExpiringJWT

func New(options jwtOptions.JWTOptions, newClaims NewClaims) JWTHandler {
	method := jwt.GetSigningMethod(options.Algorithm)
	key := options.Key

	p := jwt.NewParser(jwt.WithValidMethods([]string{method.Alg()}))
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
	p         *jwt.Parser
}

func (h *handler) New() expiringJWT.ExpiringJWT {
	return h.newClaims()
}

func (h *handler) Parse(blob string) (expiringJWT.ExpiringJWT, error) {
	t, err := h.p.ParseWithClaims(blob, h.newClaims(), h.keyFn)
	if err != nil {
		return nil, err
	}
	return t.Claims.(expiringJWT.ExpiringJWT), nil
}

func (h *handler) SetExpiryAndSign(claims expiringJWT.ExpiringJWT) (string, error) {
	claims.SetExpiry(h.expiresIn)
	t := jwt.NewWithClaims(h.method, claims)
	return t.SignedString(h.key)
}
