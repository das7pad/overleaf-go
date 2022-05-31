// Golang port of Overleaf
// Copyright (C) 2021-2022 Jakob Ackermann <das7pad@outlook.com>
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
	"fmt"
	"math"
	"os"
	"sync/atomic"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Runner func(ctx context.Context)

type Locker interface {
	RunWithLock(ctx context.Context, docId sharedTypes.UUID, runner Runner) error
	TryRunWithLock(ctx context.Context, docId sharedTypes.UUID, runner Runner) error
}

func New(client redis.UniversalClient, namespace string) (Locker, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, errors.Tag(err, "cannot get hostname")
	}
	rawRand := make([]byte, 4)
	if _, err = rand.Read(rawRand); err != nil {
		return nil, errors.Tag(err, "cannot get random salt")
	}
	rnd := hex.EncodeToString(rawRand)

	return &locker{
		client: client,

		counter:   0,
		hostname:  hostname,
		pid:       os.Getpid(),
		rnd:       rnd,
		namespace: namespace,
	}, nil
}

var ErrLocked = errors.New("locked")

type locker struct {
	client redis.UniversalClient

	counter   int64
	hostname  string
	pid       int
	rnd       string
	namespace string
}

const (
	LockTestInterval      = 50 * time.Millisecond
	MaxTestInterval       = 1 * time.Second
	MaxLockWaitTime       = 10 * time.Second
	MaxRedisRequestLength = 5 * time.Second
	LockTTL               = 30 * time.Second
)

var unlockScript = redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
	return redis.call("del", KEYS[1])
else
	return 0
end
`)

func (l *locker) getUniqueValue() string {
	now := time.Now().UnixNano()
	n := atomic.AddInt64(&l.counter, 1)
	return fmt.Sprintf(
		"locked:host=%s:pid=%d:random=%s:time=%d:count=%d",
		l.hostname, l.pid, l.rnd, now, n,
	)
}

func (l *locker) RunWithLock(ctx context.Context, docId sharedTypes.UUID, runner Runner) error {
	return l.runWithLock(ctx, docId, runner, true)
}

func (l *locker) TryRunWithLock(ctx context.Context, docId sharedTypes.UUID, runner Runner) error {
	return l.runWithLock(ctx, docId, runner, false)
}

func (l *locker) runWithLock(ctx context.Context, docId sharedTypes.UUID, runner Runner, poll bool) error {
	key := fmt.Sprintf("%s:{%s}", l.namespace, docId.String())
	lockValue := l.getUniqueValue()

	acquireLockDeadline := time.Now().Add(MaxLockWaitTime)
	acquireLockCtx, doneAcquireLock := context.WithDeadline(
		ctx, acquireLockDeadline,
	)
	defer doneAcquireLock()

	// Work that does not finish before workDeadline has the potential to
	//  overrun the lock.
	var workDeadline time.Time
	// We can be sure that the lock has expired after lockExpiredAfter.
	var lockExpiredAfter time.Time

	testInterval := LockTestInterval
	for {
		workDeadline = time.Now().Add(LockTTL)
		gotLock, timedOut, err := l.tryGetLock(acquireLockCtx, key, lockValue)
		lockExpiredAfter = time.Now().Add(LockTTL)
		if err != nil {
			err2 := l.releaseLock(key, lockValue, lockExpiredAfter)
			if poll && timedOut && err2 == nil && acquireLockCtx.Err() == nil {
				continue
			}
			return errors.Tag(err, "cannot check/acquire lock")
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
	defer workDone()
	runner(workCtx)

	return l.releaseLock(key, lockValue, lockExpiredAfter)
}

func (l *locker) tryGetLock(ctx context.Context, key string, lockValue string) (bool, bool, error) {
	getLockCtx, cancel := context.WithTimeout(ctx, MaxRedisRequestLength)
	defer cancel()

	ok, err := l.client.SetNX(getLockCtx, key, lockValue, LockTTL).Result()
	if err != nil {
		attemptTimedOut :=
			err == context.DeadlineExceeded && ctx.Err() == nil
		return false, attemptTimedOut, err
	}
	return ok, false, nil
}

func (l *locker) releaseLock(key string, lockValue string, lockExpiredAfter time.Time) error {
	if time.Now().After(lockExpiredAfter) {
		// The lock has expired. There is no need for explicit redis calls.
		return nil
	}

	keys := []string{key}
	argv := []interface{}{lockValue}

	ctx, done := context.WithDeadline(context.Background(), lockExpiredAfter)
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
