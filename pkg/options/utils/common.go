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

package utils

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

func GetIntFromEnv(key string, fallback int) int {
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

func GetStringFromEnv(key, fallback string) string {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	return raw
}

func GetBoolFromEnv(key string) bool {
	return strings.ToLower(GetStringFromEnv(key, "false")) == "true"
}

func MustGetStringFromEnv(key string) string {
	raw := os.Getenv(key)
	if raw == "" {
		panic(errors.New("missing " + key))
	}
	return raw
}

func GetDurationFromEnv(key string, fallback time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	return time.Duration(GetIntFromEnv(key, 0) * int(time.Millisecond))
}

func ParseJSONFromEnv(key string, target interface{}) {
	raw := MustGetStringFromEnv(key)
	err := json.Unmarshal([]byte(raw), target)
	if err != nil {
		panic(errors.Tag(err, "malformed "+key))
	}
}
