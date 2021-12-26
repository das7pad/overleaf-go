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
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

func ParseAndValidateId(c *gin.Context, name string) (primitive.ObjectID, *errors.ValidationError) {
	id, err := primitive.ObjectIDFromHex(c.Param(name))
	if err != nil || id == primitive.NilObjectID {
		return primitive.NilObjectID, &errors.ValidationError{
			Msg: "invalid " + name,
		}
	}
	return id, nil
}

func GetId(c *gin.Context, name string) primitive.ObjectID {
	return c.MustGet(name).(primitive.ObjectID)
}

func ValidateAndSetId(name string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := ParseAndValidateId(c, name)
		if err != nil {
			RespondErr(c, err)
			return
		}
		c.Set(name, id)
	}
}

func ValidateAndSetIdZeroOK(name string) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.Param(name)
		if raw == "000000000000000000000000" {
			c.Set(name, primitive.NilObjectID)
		} else {
			id, err := ParseAndValidateId(c, name)
			if err != nil {
				RespondErr(c, err)
				return
			}
			c.Set(name, id)
		}
	}
}
