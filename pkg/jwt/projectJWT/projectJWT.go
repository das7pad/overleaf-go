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

package projectJWT

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/httpUtils"
	"github.com/das7pad/overleaf-go/pkg/jwt/expiringJWT"
	"github.com/das7pad/overleaf-go/pkg/jwt/jwtHandler"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/options/jwtOptions"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type JWTHandler = jwtHandler.JWTHandler[*Claims]

type Claims struct {
	validateProjectJWTEpochs

	expiringJWT.Claims
	project.AuthorizationDetails
	sharedTypes.ProjectOptions
	EpochUser int64 `json:"eu"`
}

type validateProjectJWTEpochs func(ctx context.Context, projectId, userId sharedTypes.UUID, projectEpoch, userEpoch int64) error

const (
	jwtField = "projectJWT.Claims"
)

var ErrMissingPrivilegeLevel = &errors.UnauthorizedError{
	Reason: "incomplete jwt: missing PrivilegeLevel",
}

var ErrMismatchingProjectId = &errors.ValidationError{
	Msg: "mismatching projectId between jwt and path",
}

func (c *Claims) Validate(now time.Time) error {
	if err := c.Claims.Validate(now); err != nil {
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

func (c *Claims) PostProcess(target *httpUtils.Context) (*httpUtils.Context, error) {
	projectIdInPath := target.Param("projectId")
	if projectIdInPath == "" || projectIdInPath != c.ProjectId.String() {
		return target, ErrMismatchingProjectId
	}

	if err := c.CheckEpochItems(target); err != nil {
		return target, err
	}

	return target.AddValue(jwtField, c), nil
}

func MustGet(c *httpUtils.Context) *Claims {
	return c.Value(jwtField).(*Claims)
}

func New(options jwtOptions.JWTOptions, validate validateProjectJWTEpochs) *JWTHandler {
	return jwtHandler.New[*Claims](options, func() *Claims {
		return &Claims{
			validateProjectJWTEpochs: validate,
		}
	})
}

func (c *Claims) FastUnmarshalJSON(p []byte) error {
	if err := c.tryUnmarshalJSON(p); err != nil {
		return json.Unmarshal(p, c)
	}
	return nil
}

var errBadJWT = errors.New("bad jwt")

type claimField int8

const (
	claimFieldExpiresAt claimField = iota + 1
	claimFieldEpoch
	claimFieldPrivilegeLevel
	claimFieldAccessSource
	claimFieldCompileGroup
	claimFieldProjectId
	claimFieldUserId
	claimFieldTimeout
	claimFieldEpochUser
)

var claimFieldMap [256]claimField

func init() {
	claimFieldMap['c'] = claimFieldCompileGroup
	claimFieldMap['l'] = claimFieldPrivilegeLevel
	claimFieldMap['p'] = claimFieldProjectId
	claimFieldMap['s'] = claimFieldAccessSource
	claimFieldMap['t'] = claimFieldTimeout
	claimFieldMap['u'] = claimFieldUserId
}

func (c *Claims) tryUnmarshalJSON(p []byte) error {
	i := 0
	if len(p) < 2 || p[i] != '{' || p[len(p)-1] != '}' {
		return errBadJWT
	}
	i++
	for len(p) > i+3 && p[i] == '"' {
		i++
		f := claimFieldMap[p[i]]
		if p[i] == 'e' {
			switch p[i+1] {
			case 'u':
				i++
				f = claimFieldEpochUser
			case 'x':
				if p[i+2] == 'p' {
					i += 2
					f = claimFieldExpiresAt
				} else {
					return errBadJWT
				}
			case '"':
				f = claimFieldEpoch
			default:
				return errBadJWT
			}
		}
		if f == 0 {
			return errBadJWT
		}
		i++
		if len(p) < i+3 || p[i] != '"' || p[i+1] != ':' {
			return errBadJWT
		}
		i += 2
		next := bytes.IndexByte(p[i:], ',')
		j := i + next
		if next == -1 {
			j = len(p) - 1
		}
		switch f {
		case claimFieldEpoch, claimFieldEpochUser,
			claimFieldExpiresAt, claimFieldTimeout:
			v, err := strconv.ParseInt(string(p[i:j]), 10, 64)
			if err != nil {
				return errBadJWT
			}
			switch f {
			case claimFieldEpoch:
				c.Epoch = v
			case claimFieldEpochUser:
				c.EpochUser = v
			case claimFieldExpiresAt:
				c.ExpiresAt = v
			case claimFieldTimeout:
				c.Timeout = sharedTypes.ComputeTimeout(v)
			}
		case claimFieldProjectId:
			if err := c.ProjectId.UnmarshalJSON(p[i:j]); err != nil {
				return errBadJWT
			}
		case claimFieldUserId:
			if err := c.UserId.UnmarshalJSON(p[i:j]); err != nil {
				return errBadJWT
			}
		case claimFieldAccessSource:
			switch project.AccessSource(p[i+1 : j-1]) {
			case project.AccessSourceToken:
				c.AccessSource = project.AccessSourceToken
			case project.AccessSourceInvite:
				c.AccessSource = project.AccessSourceInvite
			case project.AccessSourceOwner:
				c.AccessSource = project.AccessSourceOwner
			default:
				return errBadJWT
			}
		case claimFieldPrivilegeLevel:
			switch sharedTypes.PrivilegeLevel(p[i+1 : j-1]) {
			case sharedTypes.PrivilegeLevelOwner:
				c.PrivilegeLevel = sharedTypes.PrivilegeLevelOwner
			case sharedTypes.PrivilegeLevelReadOnly:
				c.PrivilegeLevel = sharedTypes.PrivilegeLevelReadOnly
			case sharedTypes.PrivilegeLevelReadAndWrite:
				c.PrivilegeLevel = sharedTypes.PrivilegeLevelReadAndWrite
			default:
				return errBadJWT
			}
		case claimFieldCompileGroup:
			switch sharedTypes.CompileGroup(p[i+1 : j-1]) {
			case sharedTypes.StandardCompileGroup:
				c.CompileGroup = sharedTypes.StandardCompileGroup
			case sharedTypes.PriorityCompileGroup:
				c.CompileGroup = sharedTypes.PriorityCompileGroup
			default:
				return errBadJWT
			}
		}
		if next == -1 {
			return nil
		}
		i = j + 1
	}
	return errBadJWT
}
