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
	"time"

	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type EpochClaims interface {
	EpochItems() FetchJWTEpochItems
}

type FetchEpochFromMongo func(ctx context.Context, id primitive.ObjectID) (int64, error)

type JWTEpochItem struct {
	Field string
	Id    primitive.ObjectID
	Epoch *int64
	Cmd   *redis.StringCmd
	Fetch FetchEpochFromMongo
}

type JWTEpochItems []*JWTEpochItem

type FetchJWTEpochItems struct {
	Items  JWTEpochItems
	Client redis.UniversalClient
}

func (i *JWTEpochItem) Key() string {
	return "epoch:" + i.Field + ":" + i.Id.Hex()
}

func (i *JWTEpochItem) fromRedis(ctx context.Context, p redis.Pipeliner) {
	i.Cmd = p.Get(ctx, i.Key())
}

const oneDay = 24 * time.Hour

func (i *JWTEpochItem) fromMongo(ctx context.Context, p redis.Pipeliner) error {
	epoch, err := i.Fetch(ctx, i.Id)
	if err != nil {
		return errors.Tag(err, "cannot get "+i.Field+" from mongo")
	}
	_ = p.SetNX(ctx, i.Key(), epoch, oneDay)
	i.Cmd = p.Get(ctx, i.Key())
	return nil
}

func (i FetchJWTEpochItems) Check(ctx context.Context) error {
	_, err := i.Client.Pipelined(ctx, func(p redis.Pipeliner) error {
		for _, fetchJWTEpochItem := range i.Items {
			fetchJWTEpochItem.fromRedis(ctx, p)
		}
		return nil
	})
	if err != nil && err != redis.Nil {
		return errors.Tag(err, "cannot get epochs from redis")
	}
	for _, fetchJWTEpochItem := range i.Items {
		actual := fetchJWTEpochItem.Epoch
		expected, err2 := fetchJWTEpochItem.Cmd.Int64()
		if err2 != nil || actual == nil || *actual != expected {
			return &errors.UnauthorizedError{
				Reason: "epoch mismatch: " + fetchJWTEpochItem.Field,
			}
		}
	}
	return nil
}

func (i FetchJWTEpochItems) Populate(ctx context.Context) error {
	_, err := i.Client.Pipelined(ctx, func(p redis.Pipeliner) error {
		for _, fetchJWTEpochItem := range i.Items {
			fetchJWTEpochItem.fromRedis(ctx, p)
		}
		return nil
	})
	if err != nil && err != redis.Nil {
		return errors.Tag(err, "cannot get epochs from redis")
	}
	if err == redis.Nil {
		// Back-fill from mongo
		_, err = i.Client.Pipelined(ctx, func(p redis.Pipeliner) error {
			eg, pCtx := errgroup.WithContext(ctx)
			// Fetch from mongo in parallel
			for _, fetchJWTEpochItem := range i.Items {
				if fetchJWTEpochItem.Cmd.Err() == nil {
					continue
				}
				func(fetchJWTEpochItem *JWTEpochItem) {
					eg.Go(func() error {
						return fetchJWTEpochItem.fromMongo(pCtx, p)
					})
				}(fetchJWTEpochItem)
			}
			// Wait for fetch from mongo, then push/pull in parallel to redis
			if err = eg.Wait(); err != nil {
				return errors.Tag(err, "cannot fetch epochs from mongo")
			}
			return nil
		})
		if err != nil {
			return errors.Tag(err, "cannot back-fill epochs")
		}
	}
	for _, fetchJWTEpochItem := range i.Items {
		epoch, err2 := fetchJWTEpochItem.Cmd.Int64()
		if err2 != nil {
			return errors.Tag(
				err2, "cannot decode epoch for "+fetchJWTEpochItem.Field,
			)
		}
		*fetchJWTEpochItem.Epoch = epoch
		fetchJWTEpochItem.Cmd = nil
	}
	return nil
}
