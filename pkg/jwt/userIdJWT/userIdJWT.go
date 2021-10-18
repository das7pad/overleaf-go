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

package userIdJWT

import (
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/jwt/expiringJWT"
	"github.com/das7pad/overleaf-go/pkg/jwt/jwtHandler"
	"github.com/das7pad/overleaf-go/pkg/options/jwtOptions"
)

type Claims struct {
	expiringJWT.Claims
	UserId primitive.ObjectID `json:"userId"`
}

func (c *Claims) Populate(target *gin.Context) {
	target.Set("userId", c.UserId)
}

func New(options jwtOptions.JWTOptions) jwtHandler.JWTHandler {
	return jwtHandler.New(options, func() expiringJWT.ExpiringJWT {
		return &Claims{}
	})
}
