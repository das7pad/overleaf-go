// Golang port of the Overleaf linked-url-proxy service
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
	timeoutRaw := os.Getenv(key)
	if timeoutRaw == "" {
		return fallback
	}
	timeout, err := strconv.ParseInt(timeoutRaw, 10, 64)
	if err != nil {
		panic(err)
	}
	return time.Duration(timeout * int64(time.Millisecond))
}

type linkedUrlProxyOptions struct {
	address    string
	timeout    time.Duration
	proxyToken string
}

func getOptions() *linkedUrlProxyOptions {
	o := &linkedUrlProxyOptions{}

	listenAddress := getStringFromEnv("LISTEN_ADDRESS", "localhost")
	port := getIntFromEnv("PORT", 8080)
	o.address = fmt.Sprintf("%s:%d", listenAddress, port)

	o.timeout = getDurationFromEnv("LINKED_URL_PROXY_TIMEOUT", 28*time.Second)

	o.proxyToken = getStringFromEnv("PROXY_TOKEN", "")
	if o.proxyToken == "" {
		panic("missing PROXY_TOKEN")
	}
	return o
}
