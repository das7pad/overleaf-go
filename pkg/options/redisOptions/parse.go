// Golang port of Overleaf
// Copyright (C) 2021-2024 Jakob Ackermann <das7pad@outlook.com>
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

package redisOptions

import (
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/das7pad/overleaf-go/pkg/options/env"
)

func Parse() *redis.UniversalOptions {
	return &redis.UniversalOptions{
		Addrs: strings.Split(
			env.GetString("REDIS_HOST", "localhost:6379"),
			",",
		),
		Username: env.GetString("REDIS_USERNAME", ""),
		Password: env.GetString("REDIS_PASSWORD", ""),
		MaxRetries: env.GetInt(
			"REDIS_MAX_RETRIES_PER_REQUEST", 20,
		),
		PoolSize:              env.GetInt("REDIS_POOL_SIZE", 0),
		DB:                    env.GetInt("REDIS_DB", 0),
		DialTimeout:           env.GetDuration("REDIS_TIMEOUT_DIAL", 10*time.Second),
		ReadTimeout:           env.GetDuration("REDIS_TIMEOUT_READ", 10*time.Second),
		WriteTimeout:          env.GetDuration("REDIS_TIMEOUT_WRITE", 10*time.Second),
		ContextTimeoutEnabled: true,
	}
}
