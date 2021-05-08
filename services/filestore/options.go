// Golang port of the Overleaf filestore service
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

	"github.com/das7pad/filestore/pkg/backend"
	"github.com/das7pad/filestore/pkg/managers/filestore"
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

func getBoolFromEnv(key string, fallback bool) bool {
	if v, exists := os.LookupEnv(key); !exists || v == "" {
		return fallback
	}
	return os.Getenv(key) == "true"
}

func getJSONFromEnv(key string, target interface{}) {
	if v, exists := os.LookupEnv(key); !exists || v == "" {
		panic(fmt.Errorf("missing %s", key))
	}
	err := json.Unmarshal([]byte(os.Getenv(key)), target)
	if err != nil {
		panic(fmt.Errorf("malformed %s: %w", key, err))
	}
}

type filestoreOptions struct {
	address        string
	allowRedirects bool
	backendOptions backend.Options
	buckets        filestore.Buckets
}

func getOptions() *filestoreOptions {
	o := &filestoreOptions{}
	listenAddress := getStringFromEnv("LISTEN_ADDRESS", "localhost")
	port := getIntFromEnv("PORT", 3009)
	o.address = fmt.Sprintf("%s:%d", listenAddress, port)

	o.allowRedirects = getBoolFromEnv("ALLOW_REDIRECTS", false)
	getJSONFromEnv("BACKEND_OPTIONS", &o.backendOptions)
	getJSONFromEnv("BUCKETS", &o.buckets)
	return o
}
