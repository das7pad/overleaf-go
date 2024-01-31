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

package loggedInUserJWT

import (
	"bytes"
	"encoding/json"
	"strconv"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/httpUtils"
	"github.com/das7pad/overleaf-go/pkg/jwt/expiringJWT"
	"github.com/das7pad/overleaf-go/pkg/jwt/jwtHandler"
	"github.com/das7pad/overleaf-go/pkg/options/jwtOptions"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type JWTHandler jwtHandler.JWTHandler[*Claims]

type Claims struct {
	expiringJWT.Claims
	UserId sharedTypes.UUID `json:"userId"`
}

func (c *Claims) Validate(now time.Time) error {
	if err := c.Claims.Validate(now); err != nil {
		return err
	}
	if c.UserId.IsZero() {
		return &errors.NotAuthorizedError{}
	}
	return nil
}

func (c *Claims) PostProcess(target *httpUtils.Context) (*httpUtils.Context, error) {
	return target.AddValue("userId", c.UserId), nil
}

func New(options jwtOptions.JWTOptions) JWTHandler {
	return jwtHandler.New[*Claims](options, func() *Claims {
		return &Claims{}
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
	claimFieldUserId
)

func (c *Claims) tryUnmarshalJSON(p []byte) error {
	i := 0
	if len(p) < 2 || p[i] != '{' || p[len(p)-1] != '}' {
		return errBadJWT
	}
	i++
	for len(p) > i+3 && p[i] == '"' {
		var f claimField
		switch {
		case bytes.HasPrefix(p[i:], []byte(`"exp":`)):
			f = claimFieldExpiresAt
			i += 6
		case bytes.HasPrefix(p[i:], []byte(`"userId":`)):
			f = claimFieldUserId
			i += 9
		default:
			return errBadJWT
		}
		next := bytes.IndexByte(p[i:], ',')
		j := i + next
		if next == -1 {
			j = len(p) - 1
		}
		switch f {
		case claimFieldExpiresAt:
			v, err := strconv.ParseInt(string(p[i:j]), 10, 64)
			if err != nil {
				return errBadJWT
			}
			c.ExpiresAt = v
		case claimFieldUserId:
			if err := c.UserId.UnmarshalJSON(p[i:j]); err != nil {
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
