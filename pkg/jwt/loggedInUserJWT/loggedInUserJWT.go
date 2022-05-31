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

package loggedInUserJWT

import (
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/jwt/expiringJWT"
	"github.com/das7pad/overleaf-go/pkg/jwt/jwtHandler"
	"github.com/das7pad/overleaf-go/pkg/jwt/userIdJWT"
	"github.com/das7pad/overleaf-go/pkg/options/jwtOptions"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type userIdJWTClaims = userIdJWT.Claims

type Claims struct {
	userIdJWTClaims
}

func (c *Claims) Valid() error {
	if err := c.userIdJWTClaims.Valid(); err != nil {
		return err
	}
	if c.UserId == (sharedTypes.UUID{}) {
		return &errors.NotAuthorizedError{}
	}
	return nil
}

func New(options jwtOptions.JWTOptions) jwtHandler.JWTHandler {
	return jwtHandler.New(options, func() expiringJWT.ExpiringJWT {
		return &Claims{}
	})
}
