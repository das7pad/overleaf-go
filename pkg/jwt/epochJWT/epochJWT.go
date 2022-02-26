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

package epochJWT

import (
	"context"
	"log"
	"time"

	"github.com/edgedb/edgedb-go"
	"github.com/go-redis/redis/v8"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

const (
	// tombstone is an invalid epoch value used for marking a slot in redis
	//  prior to updating the epoch in mongo.
	tombstone       = -42
	expiryTombstone = 1 * time.Minute

	expiryRegular = 24 * time.Hour
)

var errEpochInFuture = &errors.InvalidStateError{Msg: "epoch in future"}

type FetchEpochFromMongo func(ctx context.Context, id edgedb.UUID) (int64, error)

type JWTEpochItem struct {
	Field             string
	Id                edgedb.UUID
	UserProvidedEpoch int64
	Fetch             FetchEpochFromMongo
	cmd               *redis.StringCmd
}

func (i *JWTEpochItem) Delete(ctx context.Context, client redis.UniversalClient) error {
	return i.scheduleReSync(ctx, client).Err()
}

func (i *JWTEpochItem) key() string {
	return "epoch:" + i.Field + ":" + i.Id.String()
}

func (i *JWTEpochItem) checkIsEpochSet() error {
	if i.UserProvidedEpoch <= 0 {
		return &errors.ValidationError{Msg: "epoch missing for " + i.Field}
	}
	return nil
}

func (i *JWTEpochItem) checkMatches(expected int64) error {
	if i.UserProvidedEpoch < expected {
		return &errors.UnauthorizedError{Reason: "epoch stale: " + i.Field}
	}
	if i.UserProvidedEpoch > expected {
		return errEpochInFuture
	}
	return nil
}

func (i *JWTEpochItem) checkInMongo(ctx context.Context) (int64, error) {
	expected, err := i.Fetch(ctx, i.Id)
	if err != nil {
		return 0, errors.Tag(err, "cannot get epoch from mongo")
	}
	if err = i.checkMatches(expected); err != nil {
		return 0, err
	}
	return expected, nil
}

func (i *JWTEpochItem) checkWithFallbackToMongo(ctx context.Context, p redis.Pipeliner) error {
	expected, err := i.cmd.Int64()
	if err == nil && expected != tombstone {
		err = i.checkMatches(expected)
		switch err {
		case nil:
			// Fast happy path.
			return nil
		case errEpochInFuture:
			// Stale epoch in redis: An operation involving the bump of the
			//  epoch overran a tombstone, or we are in a split-bain scenario.
			expected, err = i.checkInMongo(ctx)
			if err != nil {
				return err
			}
			i.scheduleReSync(ctx, p)
			return nil
		default:
			// Fast unhappy path.
			return err
		}
	}

	expected, err = i.checkInMongo(ctx)
	if err != nil {
		// Slow unhappy path.
		return err
	}
	// Slow happy path.

	if expected == tombstone {
		// Leave the tombstone alone, it will disappear soon via the TTL.
		return nil
	}

	switch i.cmd.Err() {
	case nil:
		// Corrupted value.
		i.scheduleReSync(ctx, p)
	case redis.Nil:
		// Sync now.
		i.setInRedis(ctx, p, expected)
	}
	return nil
}

func (i *JWTEpochItem) fetchFromRedis(ctx context.Context, p redis.Pipeliner) {
	i.cmd = p.Get(ctx, i.key())
}

// scheduleReSync enters a fail-safe mode for a slot. It provides a short
//  timeframe for redis and mongo to re-sync in a safe manner, e.g. ahead of
//  bumping the epoch in mongo, or fixing a stale/corrupted value.
func (i *JWTEpochItem) scheduleReSync(ctx context.Context, p redis.Cmdable) *redis.StatusCmd {
	return p.Set(ctx, i.key(), tombstone, expiryTombstone)
}

func (i *JWTEpochItem) setInRedis(ctx context.Context, p redis.Pipeliner, v int64) {
	// The slot may hold a tombstone by now. Do not overwrite.
	p.SetNX(ctx, i.key(), v, expiryRegular)
}

type JWTEpochItems []*JWTEpochItem

func (items JWTEpochItems) Check(ctx context.Context, client redis.UniversalClient) error {
	if len(items) == 0 {
		return errors.New("must provide epoch items")
	}
	for _, i := range items {
		if err := i.checkIsEpochSet(); err != nil {
			return err
		}
	}

	// Ignore any redis errors. We fall back to mongo on error.
	_, _ = client.Pipelined(ctx, func(p redis.Pipeliner) error {
		for _, fetchJWTEpochItem := range items {
			fetchJWTEpochItem.fetchFromRedis(ctx, p)
		}
		return nil
	})

	// Queue all write operations in memory.
	p := client.Pipeline()
	// Assumption: On average only one epoch changes, rendering any complexity
	//              for parallelization of the checking useless.
	for _, i := range items {
		if err := i.checkWithFallbackToMongo(ctx, p); err != nil {
			return err
		}
	}

	go func() {
		// Do not hold off user requests for back-filling into redis.
		bCtx, done := context.WithTimeout(context.Background(), 3*time.Second)
		defer done()
		if _, err := p.Exec(bCtx); err != nil {
			log.Printf("cannot back-fill epochs into redis: %s", err.Error())
		}
	}()
	return nil
}
