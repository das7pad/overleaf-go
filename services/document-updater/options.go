// Golang port of the Overleaf document-updater service
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
	"time"

	"github.com/go-redis/redis/v8"

	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"

	"github.com/das7pad/document-updater/pkg/errors"
	"github.com/das7pad/document-updater/pkg/types"
)

func getDurationFromEnv(key string, fallback time.Duration) time.Duration {
	if v, exists := os.LookupEnv(key); !exists || v == "" {
		return fallback
	}
	return time.Duration(
		getIntFromEnv(key, 0) * int64(time.Millisecond),
	)
}

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

type documentUpdaterOptions struct {
	address string

	mongoOptions *options.ClientOptions
	dbName       string
	redisOptions *redis.UniversalOptions
	options      *types.Options
}

func getOptions() *documentUpdaterOptions {
	o := &documentUpdaterOptions{}
	listenAddress := getStringFromEnv("LISTEN_ADDRESS", "localhost")
	port := getIntFromEnv("PORT", 3003)
	o.address = listenAddress + ":" + strconv.FormatInt(port, 10)

	getJSONFromEnv("OPTIONS", &o.options)
	if o.options.PendingUpdatesListShardCount <= 0 {
		panic("pending_updates_list_shard_count must be greater than 0")
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

	mongoConnectionString := os.Getenv("MONGO_CONNECTION_STRING")
	if mongoConnectionString == "" {
		mongoHost := os.Getenv("MONGO_HOST")
		if mongoHost == "" {
			mongoHost = "localhost"
		}
		mongoConnectionString = "mongodb://" + mongoHost + "/sharelatex"
	}
	mongoOptions := options.Client()
	mongoOptions.ApplyURI(mongoConnectionString)
	mongoOptions.SetAppName(os.Getenv("SERVICE_NAME"))
	mongoOptions.SetMaxPoolSize(
		uint64(getIntFromEnv("MONGO_POOL_SIZE", 10)),
	)
	mongoOptions.SetSocketTimeout(
		getDurationFromEnv("MONGO_SOCKET_TIMEOUT", 30*time.Second),
	)
	mongoOptions.SetServerSelectionTimeout(getDurationFromEnv(
		"MONGO_SERVER_SELECTION_TIMEOUT",
		60*time.Second,
	))
	o.mongoOptions = mongoOptions

	cs, err := connstring.Parse(mongoConnectionString)
	if err != nil {
		panic(err)
	}
	o.dbName = cs.Database

	return o
}
