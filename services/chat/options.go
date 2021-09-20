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

package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
)

func getIntFromEnv(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		panic(err)
	}
	return int(parsed)
}

func getStringFromEnv(key, fallback string) string {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	return raw
}

func getDurationFromEnv(key string, fallback time.Duration) time.Duration {
	if v, exists := os.LookupEnv(key); !exists || v == "" {
		return fallback
	}
	return time.Duration(getIntFromEnv(key, 0) * int(time.Millisecond))
}

func getOptions() (
	address string,
	mongoOptions *options.ClientOptions,
	dbName string,
) {
	listenAddress := getStringFromEnv("LISTEN_ADDRESS", "localhost")
	port := getIntFromEnv("PORT", 3010)
	address = fmt.Sprintf("%s:%d", listenAddress, port)

	mongoConnectionString := os.Getenv("MONGO_CONNECTION_STRING")
	if mongoConnectionString == "" {
		mongoHost := os.Getenv("MONGO_HOST")
		if mongoHost == "" {
			mongoHost = "localhost"
		}
		mongoConnectionString = fmt.Sprintf(
			"mongodb://%s/sharelatex", mongoHost,
		)
	}
	mongoOptions = options.Client()
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

	cs, err := connstring.Parse(mongoConnectionString)
	if err != nil {
		panic(err)
	}
	dbName = cs.Database
	return
}
