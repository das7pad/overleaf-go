// Golang port of Overleaf
// Copyright (C) 2024 Jakob Ackermann <das7pad@outlook.com>
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

package redisScanner

import (
	"context"

	"github.com/go-redis/redis/v8"
	"golang.org/x/sync/errgroup"
)

func ScanRedis(ctx context.Context, uc redis.UniversalClient, match string, count int64) (chan string, chan error) {
	s := scanner{
		q:     make(chan string, count),
		err:   make(chan error),
		match: match,
		count: count,
	}
	s.eg, ctx = errgroup.WithContext(ctx)
	go s.run(ctx, uc)
	return s.q, s.err
}

type scannerClient interface {
	Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd
}

type scanner struct {
	q     chan string
	err   chan error
	eg    *errgroup.Group
	match string
	count int64
}

func (s *scanner) run(ctx context.Context, uc redis.UniversalClient) {
	go func() {
		<-ctx.Done()
		for range s.q {
			// flush queue on context cancellation
		}
	}()
	var err error
	if c, ok := uc.(*redis.ClusterClient); ok {
		err = s.scanCluster(ctx, c)
	} else {
		err = s.scan(ctx, uc)
	}
	err2 := s.eg.Wait()

	if err != nil {
		s.err <- err
	} else if err2 != nil {
		s.err <- err2
	}
	close(s.q)
	close(s.err)
}

func (s *scanner) scanCluster(ctx context.Context, c *redis.ClusterClient) error {
	return c.ForEachMaster(ctx, func(ctx context.Context, c *redis.Client) error {
		return s.scan(ctx, c)
	})
}

func (s *scanner) scan(ctx context.Context, sc scannerClient) error {
	items, cur, err := sc.Scan(ctx, 0, s.match, s.count).Result()
	if err != nil {
		return err
	}
	s.eg.Go(func() error {
		return s.scanLoop(ctx, sc, items, cur)
	})
	return nil
}

func (s *scanner) scanLoop(ctx context.Context, sc scannerClient, items []string, cur uint64) error {
	for {
		for _, item := range items {
			s.q <- item
		}
		if cur == 0 {
			return nil
		}
		var err error
		items, cur, err = sc.Scan(ctx, cur, s.match, s.count).Result()
		if err != nil {
			return err
		}
	}
}
