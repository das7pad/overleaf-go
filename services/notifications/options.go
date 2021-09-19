// Golang port of the Overleaf notifications service
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
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"

	"github.com/das7pad/overleaf-go/pkg/httpUtils"
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

type notificationsOptions struct {
	address      string
	corsOptions  httpUtils.CORSOptions
	jwtOptions   httpUtils.JWTOptions
	mongoOptions *options.ClientOptions
	dbName       string
}

func getOptions() *notificationsOptions {
	o := &notificationsOptions{}
	listenAddress := getStringFromEnv("LISTEN_ADDRESS", "localhost")
	port := getIntFromEnv("PORT", 3042)
	o.address = fmt.Sprintf("%s:%d", listenAddress, port)

	jwtSecret := os.Getenv("JWT_NOTIFICATIONS_VERIFY_SECRET")
	if jwtSecret == "" {
		panic("missing JWT_NOTIFICATIONS_VERIFY_SECRET")
	}
	o.jwtOptions.Algorithm = "HS512"
	o.jwtOptions.Key = jwtSecret

	siteUrl := getStringFromEnv("PUBLIC_URL", "http://localhost:3000")
	allowOrigins := strings.Split(
		getStringFromEnv("ALLOWED_ORIGINS", siteUrl),
		",",
	)
	o.corsOptions.AllowOrigins = allowOrigins

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
	o.mongoOptions = options.Client()
	o.mongoOptions.ApplyURI(mongoConnectionString)
	o.mongoOptions.SetAppName(os.Getenv("SERVICE_NAME"))
	o.mongoOptions.SetMaxPoolSize(
		uint64(getIntFromEnv("MONGO_POOL_SIZE", 10)),
	)
	o.mongoOptions.SetSocketTimeout(
		getDurationFromEnv("MONGO_SOCKET_TIMEOUT", 30*time.Second),
	)
	o.mongoOptions.SetServerSelectionTimeout(getDurationFromEnv(
		"MONGO_SERVER_SELECTION_TIMEOUT",
		60*time.Second,
	))

	cs, err := connstring.Parse(mongoConnectionString)
	if err != nil {
		panic(err)
	}
	o.dbName = cs.Database
	return o
}
