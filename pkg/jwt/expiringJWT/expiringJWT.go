// Golang port of Overleaf
// Copyright (C) 2021-2024 Jakob Ackermann <das7pad@outlook.com>
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

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type ExpiringJWT interface {
	Validate(now time.Time) error
	SetExpiry(expiresIn time.Duration)
	FastUnmarshalJSON(p []byte) error
}

var ErrExpired = &errors.UnauthorizedError{Reason: "jwt expired"}

type Claims struct {
	ExpiresAt int64 `json:"exp"`
}

func (j *Claims) SetExpiry(expiresIn time.Duration) {
	j.ExpiresAt = time.Now().Add(expiresIn).Unix()
}

func (j *Claims) Validate(now time.Time) error {
	expiresAt := time.Unix(j.ExpiresAt, 0)
	if now.After(expiresAt) {
		return ErrExpired
	}
	return nil
}
