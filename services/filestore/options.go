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

	"github.com/das7pad/overleaf-go/services/filestore/pkg/types"
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
	address string
	options types.Options
}

func getOptions() *filestoreOptions {
	o := &filestoreOptions{}
	listenAddress := getStringFromEnv("LISTEN_ADDRESS", "localhost")
	port := getIntFromEnv("PORT", 3009)
	o.address = fmt.Sprintf("%s:%d", listenAddress, port)

	getJSONFromEnv("OPTIONS", &o.options)
	return o
}
