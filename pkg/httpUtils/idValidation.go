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

package httpUtils

import (
	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

func ParseAndValidateId(c *Context, name string) (edgedb.UUID, error) {
	id, err := edgedb.ParseUUID(c.Param(name))
	if err != nil || id == (edgedb.UUID{}) {
		return edgedb.UUID{}, &errors.ValidationError{
			Msg: "invalid " + name,
		}
	}
	return id, nil
}

func GetId(c *Context, name string) edgedb.UUID {
	return c.Value(name).(edgedb.UUID)
}

func ValidateAndSetId(name string) MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(c *Context) {
			id, err := ParseAndValidateId(c, name)
			if err != nil {
				RespondErr(c, err)
				return
			}
			c.AddValue(name, id)
			next(c)
		}
	}
}

func ValidateAndSetIdZeroOK(name string) MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		handleRegularId := ValidateAndSetId(name)(next)
		return func(c *Context) {
			raw := c.Param(name)
			if raw == "00000000-0000-0000-0000-000000000000" {
				c.AddValue(name, edgedb.UUID{})
				next(c)
			} else {
				handleRegularId(c)
			}
		}
	}
}
