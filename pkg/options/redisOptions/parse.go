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

package redisOptions

import (
	"strings"

	"github.com/go-redis/redis/v8"

	"github.com/das7pad/overleaf-go/pkg/options/utils"
)

func Parse() *redis.UniversalOptions {
	return &redis.UniversalOptions{
		Addrs: strings.Split(
			utils.GetStringFromEnv("REDIS_HOST", "localhost:6379"),
			",",
		),
		Password: utils.GetStringFromEnv("REDIS_PASSWORD", ""),
		MaxRetries: utils.GetIntFromEnv(
			"REDIS_MAX_RETRIES_PER_REQUEST", 20,
		),
		PoolSize: utils.GetIntFromEnv("REDIS_POOL_SIZE", 0),
	}
}
