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

package projectJWT

import (
	"context"

	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/httpUtils"
	"github.com/das7pad/overleaf-go/pkg/jwt/expiringJWT"
	"github.com/das7pad/overleaf-go/pkg/jwt/jwtHandler"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/options/jwtOptions"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Claims struct {
	validateProjectJWTEpochs

	expiringJWT.Claims
	project.AuthorizationDetails
	types.SignedCompileProjectRequestOptions
	EpochUser int64 `json:"eu"`
}

type validateProjectJWTEpochs func(ctx context.Context, projectId, userId edgedb.UUID, projectEpoch, userEpoch int64) error

const (
	jwtField = "projectJWT.Claims"
)

var ErrMissingPrivilegeLevel = errors.New(
	"incomplete jwt: missing PrivilegeLevel",
)

var ErrMismatchingProjectId = &errors.ValidationError{
	Msg: "mismatching projectId between jwt and path",
}

func (c *Claims) Valid() error {
	if err := c.Claims.Valid(); err != nil {
		return err
	}
	if c.AuthorizationDetails.PrivilegeLevel == "" {
		return ErrMissingPrivilegeLevel
	}
	return nil
}

func (c *Claims) CheckEpochItems(ctx context.Context) error {
	return c.validateProjectJWTEpochs(
		ctx, c.ProjectId, c.UserId, c.Epoch, c.EpochUser,
	)
}

func (c *Claims) PostProcess(target *httpUtils.Context) error {
	projectIdInPath := target.Param("projectId")
	if projectIdInPath == "" || projectIdInPath != c.ProjectId.String() {
		return ErrMismatchingProjectId
	}

	if err := c.CheckEpochItems(target); err != nil {
		return err
	}

	target.AddValue(jwtField, c)
	return nil
}

func MustGet(c *httpUtils.Context) *Claims {
	return c.Value(jwtField).(*Claims)
}

func New(options jwtOptions.JWTOptions, validate validateProjectJWTEpochs) jwtHandler.JWTHandler {
	return jwtHandler.New(options, func() expiringJWT.ExpiringJWT {
		return &Claims{
			validateProjectJWTEpochs: validate,
		}
	})
}
