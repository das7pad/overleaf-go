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
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

const (
	validatedIds = "httpUtils.validatedIds"
)

func GetId(c *gin.Context, name string) primitive.ObjectID {
	return c.MustGet(name).(primitive.ObjectID)
}

func ValidateAndSetId(name string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := primitive.ObjectIDFromHex(c.Param(name))
		if err != nil || id == primitive.NilObjectID {
			c.String(http.StatusBadRequest, "invalid "+name)
			c.Abort()
			return
		}
		c.Set(name, id)
		c.Next()
	}
}

func ValidateAndSetIdZeroOK(name string) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.Param(name)
		if raw == "000000000000000000000000" {
			c.Set(name, primitive.NilObjectID)
		} else {
			id, err := primitive.ObjectIDFromHex(raw)
			if err != nil || id == primitive.NilObjectID {
				c.String(http.StatusBadRequest, "invalid "+name)
				c.Abort()
				return
			}
			c.Set(name, id)
		}
		c.Next()
	}
}

func ValidateAndSetJWTId(name string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := GetIdFromJwt(c, name)
		if err != nil {
			RespondErr(c, err)
			return
		}
		if rawPathId, exists := c.Params.Get(name); exists {
			if rawPathId != id.Hex() {
				RespondErr(c, &errors.ValidationError{
					Msg: "jwt id mismatches path id: " + name,
				})
				return
			}
		}
		ids := c.GetStringSlice(validatedIds)
		ids = append(ids, name)
		c.Set(validatedIds, ids)
		c.Set(name, id)
		c.Next()
	}
}

func CheckEpochs(client redis.UniversalClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		ids := c.Value(validatedIds).([]string)
		epochs := make(map[string]*redis.StringCmd)
		_, err := client.Pipelined(c, func(p redis.Pipeliner) error {
			for _, field := range ids {
				epochs[field] = p.Get(
					c,
					"epoch:"+string(field)+":"+GetId(c, field).Hex(),
				)
			}
			return nil
		})
		if err != nil {
			RespondErr(c, errors.Tag(err, "cannot validate epoch"))
			return
		}
		for _, field := range ids {
			stored := epochs[field].Val()
			provided, err2 := GetStringFromJwt(c, "epoch_"+field)
			if err2 != nil || stored != provided {
				RespondErr(c, &errors.UnauthorizedError{
					Reason: "epoch mismatch: " + field,
				})
				return
			}
		}
		c.Next()
	}
}
