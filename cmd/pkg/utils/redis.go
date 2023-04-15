// Golang port of Overleaf
// Copyright (C) 2021-2023 Jakob Ackermann <das7pad@outlook.com>
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
	"context"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/options/redisOptions"
)

func ensureRedisAcceptsWrites(ctx context.Context, rClient redis.UniversalClient) error {
	// Write a dummy value as health check on startup.
	// Redis standalone: not reachable -> timeout
	// Redis cluster: not reachable -> timeout
	// Redis cluster: some shard is down -> error
	//                (provided `cluster-require-full-coverage=yes` is set in
	//                 redis config -- this is the default)
	return rClient.Set(ctx, "startup", "42", time.Second).Err()
}

func MustConnectRedis(ctx context.Context) redis.UniversalClient {
	ctx, done := context.WithTimeout(ctx, 10*time.Second)
	defer done()

	rOptions := redisOptions.Parse()

	rClient := redis.NewUniversalClient(rOptions)
	if err := ensureRedisAcceptsWrites(ctx, rClient); err != nil {
		panic(errors.Tag(err, "ensure redis accepts writes"))
	}
	return rClient
}
