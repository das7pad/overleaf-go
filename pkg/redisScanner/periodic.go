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
	"log"
	"math/rand"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

func Periodic(ctx context.Context, uc redis.UniversalClient, prefix string, pc PeriodicOptions, msg string, fn func(ctx context.Context, id sharedTypes.UUID) bool) {
	inter := pc.Interval
	t := time.NewTicker(inter - time.Duration(rand.Int63n(int64(inter))))
	defer t.Stop()
	done := ctx.Done()
	for {
		t0 := time.Now()
		ok, err := Each(ctx, uc, prefix, pc.Count, fn)
		d := time.Since(t0)
		log.Printf("%s: ok=%t d=%s err=%v", msg, ok, d, err)
		if ok && err == nil {
			t.Reset(inter)
		} else {
			t.Reset(inter / 2)
		}
		select {
		case <-t.C:
		case <-done:
			return
		}
	}
}

func Each(ctx context.Context, uc redis.UniversalClient, prefix string, count int64, fn func(ctx context.Context, id sharedTypes.UUID) bool) (bool, error) {
	ctx, done := context.WithCancel(ctx)
	defer done()
	keys, scanErr := ScanRedis(ctx, uc, prefix+"*", count)
	var projectId sharedTypes.UUID
	ok := true
	prefixLen := len(prefix)
	buf := make([]byte, 36)
	for key := range keys {
		if len(key) < prefixLen+36 {
			log.Printf("redisScanner: found unexpected key: %q: length", key)
			continue
		}
		copy(buf, key[prefixLen:prefixLen+36])
		if err := projectId.UnmarshalText(buf); err != nil {
			log.Printf("redisScanner: found unexpected key: %q: %s", key, err)
			continue
		}
		if !fn(ctx, projectId) {
			ok = false
		}
	}
	if err := <-scanErr; err != nil {
		return ok, errors.Tag(err, "scan")
	}
	return ok, nil
}

type PeriodicOptions struct {
	Count    int64         `json:"count"`
	Interval time.Duration `json:"interval"`
}

func (o *PeriodicOptions) Validate() error {
	if o.Count == 0 {
		return &errors.ValidationError{
			Msg: "count must be greater than 0",
		}
	}
	if o.Interval == 0 {
		return &errors.ValidationError{
			Msg: "interval must be greater than 0",
		}
	}
	return nil
}
