// Golang port of the Overleaf real-time service
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

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	jwtMiddleware "github.com/auth0/go-jwt-middleware"
	"github.com/form3tech-oss/jwt-go"

	"github.com/das7pad/real-time/pkg/errors"
	"github.com/das7pad/real-time/pkg/types"
)

func getIntFromEnv(key string, fallback int64) int64 {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		panic(err)
	}
	return parsed
}

func getStringFromEnv(key, fallback string) string {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	return raw
}

func getJSONFromEnv(key string, target interface{}) {
	if v, exists := os.LookupEnv(key); !exists || v == "" {
		panic(errors.New("missing " + key))
	}
	err := json.Unmarshal([]byte(os.Getenv(key)), target)
	if err != nil {
		panic(fmt.Errorf("malformed %s: %w", key, err))
	}
}

type clsiOptions struct {
	address string

	jwtOptions jwtMiddleware.Options
	options    *types.Options
}

func getOptions() *clsiOptions {
	o := &clsiOptions{}
	listenAddress := getStringFromEnv("LISTEN_ADDRESS", "localhost")
	port := getIntFromEnv("PORT", 13013)
	o.address = listenAddress + ":" + strconv.FormatInt(port, 10)

	getJSONFromEnv("OPTIONS", &o.options)

	jwtSecret := os.Getenv("JWT_REAL_TIME_VERIFY_SECRET")
	if jwtSecret == "" {
		panic("missing JWT_REAL_TIME_VERIFY_SECRET")
	}
	o.jwtOptions = jwtMiddleware.Options{
		ValidationKeyGetter: func(token *jwt.Token) (interface{}, error) {
			return []byte(jwtSecret), nil
		},
		SigningMethod: jwt.SigningMethodHS512,
	}
	return o
}
