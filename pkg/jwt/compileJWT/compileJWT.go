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

package compileJWT

import (
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/jwt/epochJWT"
	"github.com/das7pad/overleaf-go/pkg/jwt/expiringJWT"
	"github.com/das7pad/overleaf-go/pkg/jwt/jwtHandler"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/options/jwtOptions"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Claims struct {
	expiringJWT.Claims
	project.AuthorizationDetails
	types.SignedCompileProjectRequestOptions
	EpochUser int64 `json:"eu"`

	fetchProjectEpoch epochJWT.FetchEpochFromMongo
	fetchUserEpoch    epochJWT.FetchEpochFromMongo
	client            redis.UniversalClient
}

const (
	userIdField    = "userId"
	projectIdField = "projectId"

	jwtField = "compileJWT.Claims"
)

var ErrMissingPrivilegeLevel = errors.New(
	"incomplete jwt: missing PrivilegeLevel",
)

func (c *Claims) Valid() error {
	if err := c.Claims.Valid(); err != nil {
		return err
	}
	if c.AuthorizationDetails.PrivilegeLevel == "" {
		return ErrMissingPrivilegeLevel
	}
	return nil
}

func (c *Claims) EpochItems() epochJWT.FetchJWTEpochItems {
	return epochJWT.FetchJWTEpochItems{
		Items: epochJWT.JWTEpochItems{
			{
				Field: projectIdField,
				Id:    c.ProjectId,
				Epoch: &c.AuthorizationDetails.Epoch,
				Fetch: c.fetchProjectEpoch,
			},
			{
				Field: userIdField,
				Id:    c.UserId,
				Epoch: &c.EpochUser,
				Fetch: c.fetchUserEpoch,
			},
		},
		Client: c.client,
	}
}

func (c *Claims) Populate(target *gin.Context) {
	target.Set(jwtField, c)
}

func MustGet(ctx *gin.Context) *Claims {
	return ctx.MustGet(jwtField).(*Claims)
}

func New(options jwtOptions.JWTOptions, fetchProjectEpoch, fetchUserEpoch epochJWT.FetchEpochFromMongo, client redis.UniversalClient) jwtHandler.JWTHandler {
	return jwtHandler.New(options, func() expiringJWT.ExpiringJWT {
		return &Claims{
			fetchProjectEpoch: fetchProjectEpoch,
			fetchUserEpoch:    fetchUserEpoch,
			client:            client,
		}
	})
}
