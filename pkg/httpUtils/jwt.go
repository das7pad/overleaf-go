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
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

const (
	jwtCacheKey = "httpUtils.jwtClaims"
)

func GetJWTClaims(c *gin.Context) jwt.MapClaims {
	raw := c.MustGet(jwtCacheKey)
	return raw.(jwt.MapClaims)
}

func GetStringFromJwt(c *gin.Context, name string) (string, error) {
	claims := GetJWTClaims(c)
	if item, exists := claims[name]; exists || item != nil {
		if s, ok := item.(string); ok {
			return s, nil
		}
	}
	return "", &errors.UnauthorizedError{
		Reason: "missing jwt entry " + name,
	}
}

func GetDurationFromJwt(c *gin.Context, name string) (time.Duration, error) {
	raw, err := GetStringFromJwt(c, name)
	if err != nil {
		return 0, err
	}
	return time.ParseDuration(raw)
}

func GetIdFromJwt(c *gin.Context, name string) (primitive.ObjectID, error) {
	rawId, err := GetStringFromJwt(c, name)
	if err != nil || rawId == "" {
		return primitive.NilObjectID, &errors.UnauthorizedError{
			Reason: "missing jwt entry for " + name,
		}
	}
	if rawId == "000000000000000000000000" {
		// Allow explicitly signed all-zero id.
		return primitive.NilObjectID, nil
	}
	id, err := primitive.ObjectIDFromHex(rawId)
	if err != nil || id == primitive.NilObjectID {
		// Reject on error or parse-to-all-zero-id.
		return primitive.NilObjectID, &errors.UnauthorizedError{
			Reason: "invalid " + name,
		}
	}
	return id, nil
}

type JWTOptions struct {
	Algorithm string `json:"algo"`
	Key       string `json:"key"`
	FromQuery string `json:"from_query"`
}

func NewJWTHandler(options JWTOptions) *JWTHandler {
	parser := jwt.Parser{
		ValidMethods: []string{options.Algorithm},
	}
	keyBlob := []byte(options.Key)
	var keyFn jwt.Keyfunc = func(_ *jwt.Token) (interface{}, error) {
		return keyBlob, nil
	}
	return &JWTHandler{
		parser:    parser,
		keyFn:     keyFn,
		fromQuery: options.FromQuery,
	}
}

type JWTHandler struct {
	parser    jwt.Parser
	keyFn     jwt.Keyfunc
	fromQuery string
}

func (h *JWTHandler) Parse(c *gin.Context, claims jwt.Claims) (jwt.Claims, error) {
	var blob string
	if h.fromQuery != "" {
		blob = c.Query(h.fromQuery)
	} else {
		v := c.GetHeader("Authorization")
		if len(v) > 7 && v[:7] == "Bearer " {
			blob = v[7:]
		}
	}

	if blob == "" {
		return nil, &errors.UnauthorizedError{Reason: "missing jwt"}
	}

	t, err := h.parser.ParseWithClaims(blob, claims, h.keyFn)
	if err != nil {
		return nil, &errors.UnauthorizedError{Reason: "invalid jwt"}
	}

	c.Set(jwtCacheKey, t.Claims)
	return t.Claims, nil
}

func (h *JWTHandler) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		_, err := h.Parse(c, jwt.MapClaims{})
		if err != nil {
			RespondErr(c, err)
			return
		}
	}
}
