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
	"os"
	"strconv"
	"strings"

	jwtMiddleware "github.com/auth0/go-jwt-middleware"
	"github.com/form3tech-oss/jwt-go"
	"github.com/go-redis/redis/v8"

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
		panic(errors.Tag(err, "malformed "+key))
	}
}

type realTimeOptions struct {
	address string

	jwtOptions   jwtMiddleware.Options
	redisOptions *redis.UniversalOptions
	options      *types.Options
}

func getOptions() *realTimeOptions {
	o := &realTimeOptions{}
	listenAddress := getStringFromEnv("LISTEN_ADDRESS", "localhost")
	port := getIntFromEnv("PORT", 3026)
	o.address = listenAddress + ":" + strconv.FormatInt(port, 10)

	getJSONFromEnv("OPTIONS", &o.options)
	if o.options.PendingUpdatesListShardCount <= 0 {
		panic("pending_updates_list_shard_count must be greater than 0")
	}

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

	o.redisOptions = &redis.UniversalOptions{
		Addrs: strings.Split(
			getStringFromEnv("REDIS_HOST", "localhost:6379"),
			",",
		),
		Password: os.Getenv("REDIS_PASSWORD"),
		MaxRetries: int(getIntFromEnv(
			"REDIS_MAX_RETRIES_PER_REQUEST", 20),
		),
		PoolSize: int(getIntFromEnv("REDIS_POOL_SIZE", 0)),
	}

	return o
}
