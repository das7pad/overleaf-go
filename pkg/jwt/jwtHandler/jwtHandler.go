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

package jwtHandler

import (
	"time"

	"github.com/golang-jwt/jwt/v4"

	"github.com/das7pad/overleaf-go/pkg/jwt/expiringJWT"
	"github.com/das7pad/overleaf-go/pkg/options/jwtOptions"
)

type JWT = expiringJWT.ExpiringJWT

type JWTHandler[T JWT] interface {
	New() T
	Parse(blob string) (T, error)
	GetExpiresIn() time.Duration
	SetExpiryAndSign(claims T) (string, error)
}

type NewClaims[T JWT] func() T

func New[T JWT](options jwtOptions.JWTOptions, newClaims NewClaims[T]) JWTHandler[T] {
	method := jwt.GetSigningMethod(options.Algorithm)
	key := []byte(options.Key)

	p := jwt.NewParser(jwt.WithValidMethods([]string{method.Alg()}))
	var keyFn jwt.Keyfunc = func(_ *jwt.Token) (interface{}, error) {
		return key, nil
	}
	return &handler[T]{
		expiresIn: options.ExpiresIn,
		newClaims: newClaims,
		key:       key,
		keyFn:     keyFn,
		method:    method,
		p:         p,
	}
}

type handler[T JWT] struct {
	expiresIn time.Duration
	key       interface{}
	keyFn     jwt.Keyfunc
	method    jwt.SigningMethod
	newClaims NewClaims[T]
	p         *jwt.Parser
}

func (h *handler[T]) New() T {
	return h.newClaims()
}

func (h *handler[T]) Parse(blob string) (T, error) {
	t, err := h.p.ParseWithClaims(blob, h.newClaims(), h.keyFn)
	if err != nil {
		var tt T
		return tt, err
	}
	return t.Claims.(T), nil
}

func (h *handler[T]) GetExpiresIn() time.Duration {
	return h.expiresIn
}

func (h *handler[T]) SetExpiryAndSign(claims T) (string, error) {
	claims.SetExpiry(h.expiresIn)
	t := jwt.NewWithClaims(h.method, claims)
	return t.SignedString(h.key)
}
