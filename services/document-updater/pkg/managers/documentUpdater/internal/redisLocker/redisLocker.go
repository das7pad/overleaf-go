// Golang port of the Overleaf document-updater service
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

package redisLocker

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"math"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type Runner func(ctx context.Context)

type Locker interface {
	RunWithLock(ctx context.Context, docId primitive.ObjectID, runner Runner) error
	TryRunWithLock(ctx context.Context, docId primitive.ObjectID, runner Runner) error
}

func New(client redis.UniversalClient) Locker {
	return &locker{client: client}
}

var ErrLocked = errors.New("locked")

type locker struct {
	client redis.UniversalClient
}

const (
	LockTestInterval      = 50 * time.Millisecond
	MaxTestInterval       = 1 * time.Second
	MaxLockWaitTime       = 10 * time.Second
	MaxRedisRequestLength = 5 * time.Second
	LockTTL               = 30 * time.Second
)

var (
	hostname string
	pid      = strconv.FormatInt(int64(os.Getpid()), 10)
	rnd      string
	counter  int64 = 0
)

func init() {
	var err error
	hostname, err = os.Hostname()
	if err != nil {
		panic(errors.Tag(err, "cannot get hostname"))
	}
	rawRand := make([]byte, 4)
	_, err = rand.Read(rawRand)
	if err != nil {
		panic(errors.Tag(err, "cannot get random salt"))
	}
	rnd = hex.EncodeToString(rawRand)
}

var unlockScript = redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
	return redis.call("del", KEYS[1])
else
	return 0
end
`)

func getUniqueValue() string {
	now := strconv.FormatInt(time.Now().UnixNano(), 10)
	c := strconv.FormatInt(atomic.AddInt64(&counter, 1), 10)
	return "locked" +
		":host=" + hostname +
		":pid=" + pid +
		":random=" + rnd +
		":time=" + now +
		":count=" + c
}

func getBlockingKey(docId primitive.ObjectID) string {
	return "Blocking:{" + docId.Hex() + "}"
}

func (l *locker) RunWithLock(ctx context.Context, docId primitive.ObjectID, runner Runner) error {
	return l.runWithLock(ctx, docId, runner, true)
}

func (l *locker) TryRunWithLock(ctx context.Context, docId primitive.ObjectID, runner Runner) error {
	return l.runWithLock(ctx, docId, runner, false)
}

func (l *locker) runWithLock(ctx context.Context, docId primitive.ObjectID, runner Runner, poll bool) error {
	key := getBlockingKey(docId)
	lockValue := getUniqueValue()

	acquireLockDeadline := time.Now().Add(MaxLockWaitTime)
	acquireLockCtx, doneAcquireLock := context.WithDeadline(
		ctx, acquireLockDeadline,
	)
	defer doneAcquireLock()
	var workDeadline time.Time
	testInterval := LockTestInterval

	for {
		var gotLock bool
		var err error
		workDeadline, gotLock, err = l.getLock(acquireLockCtx, key, lockValue)
		if err != nil {
			return err
		}
		if gotLock {
			break
		}
		if !poll {
			return ErrLocked
		}
		if time.Now().Add(testInterval).After(acquireLockDeadline) {
			return context.DeadlineExceeded
		}
		time.Sleep(testInterval)
		testInterval = time.Duration(
			math.Max(float64(testInterval*2), float64(MaxTestInterval)),
		)
	}
	doneAcquireLock()

	workCtx, workDone := context.WithDeadline(ctx, workDeadline)
	runner(workCtx)
	workDone()

	if time.Now().After(workDeadline) {
		// Redis value has expired. There is not need for explicit redis calls.
		return nil
	}

	return l.releaseLock(key, lockValue, workDeadline)
}

func (l *locker) getLock(ctx context.Context, key string, lockValue string) (time.Time, bool, error) {
	workDeadline := time.Now().Add(LockTTL)
	getLockCtx, cancel := context.WithTimeout(ctx, MaxRedisRequestLength)
	defer cancel()

	ok, err := l.client.SetNX(getLockCtx, key, lockValue, LockTTL).Result()
	if err != nil {
		err2 := l.releaseLock(key, lockValue, workDeadline)
		if err == context.DeadlineExceeded && ctx.Err() == nil {
			return workDeadline, false, err2
		}
		return workDeadline, false, errors.Tag(err, "cannot check/acquire lock")
	}
	return workDeadline, ok, nil
}

func (l *locker) releaseLock(key string, lockValue string, workDeadline time.Time) error {
	keys := []string{key}
	argv := []interface{}{lockValue}

	ctx, done := context.WithDeadline(context.Background(), workDeadline)
	defer done()
	res, err := unlockScript.Run(ctx, l.client, keys, argv).Result()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			// Release request timed out, but the redis value expired as well.
			return nil
		}
		return err
	}
	switch returnValue := res.(type) {
	case int64:
		if returnValue == 1 {
			return nil
		}
		return errors.New("tried to release expired lock")
	default:
		return errors.New("release script turned unexpected value")
	}
}
