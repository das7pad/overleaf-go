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

package httpUtils

import (
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/m2pq"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

func ParseAndValidateId(c *Context, name string) (sharedTypes.UUID, error) {
	id, err := m2pq.ParseID(c.Param(name))
	if err != nil || id.IsZero() {
		return sharedTypes.UUID{}, &errors.ValidationError{
			Msg: "invalid " + name,
		}
	}
	return id, nil
}

func GetId(c *Context, name string) sharedTypes.UUID {
	return c.Value(name).(sharedTypes.UUID)
}

func ValidateAndSetId(name string) MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(c *Context) {
			id, err := ParseAndValidateId(c, name)
			if err != nil {
				RespondErr(c, err)
				return
			}
			next(c.AddValue(name, id))
		}
	}
}

func ValidateAndSetIdZeroOK(name string) MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		handleRegularId := ValidateAndSetId(name)(next)
		return func(c *Context) {
			raw := c.Param(name)
			if raw == sharedTypes.AllZeroUUID {
				next(c.AddValue(name, sharedTypes.UUID{}))
			} else {
				handleRegularId(c)
			}
		}
	}
}
