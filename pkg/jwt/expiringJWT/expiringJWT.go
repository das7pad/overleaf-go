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

package expiringJWT

import (
	"time"

	"github.com/golang-jwt/jwt/v4"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type ExpiringJWT interface {
	jwt.Claims
	SetExpiry(expiresIn time.Duration)
}

var Expired = errors.New("jwt expired")

type Claims struct {
	expiresIn time.Duration
	ExpiresAt int64 `json:"exp"`
}

func (j *Claims) SetExpiry(expiresIn time.Duration) {
	j.expiresIn = expiresIn
	j.ExpiresAt = time.Now().Add(expiresIn).Unix()
}

func (j Claims) ExpiresIn() time.Duration {
	return j.expiresIn
}

// Valid validates the given expiry timestamp.
// jwt.StandardClaims.Valid() ignores a timestamp of 0.
func (j *Claims) Valid() error {
	now := time.Now()
	expiresAt := time.Unix(j.ExpiresAt, 0)
	if now.After(expiresAt) {
		return Expired
	}
	return nil
}
