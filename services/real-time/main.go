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

package main

import (
	"context"
	"net/http"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v4/pgxpool"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/options/postgresOptions"
	"github.com/das7pad/overleaf-go/pkg/pendingOperation"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/managers/realTime"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/router"
)

func waitForRedis(
	ctx context.Context,
	rClient redis.UniversalClient,
) error {
	// Write a dummy value as health check on startup.
	// Redis standalone: not reachable -> timeout
	// Redis cluster: not reachable -> timeout; some shard down -> error *
	// *provided cluster-require-full-coverage=yes (which is the default)
	return rClient.Set(ctx, "startup", "42", time.Second).Err()
}

func main() {
	o := getOptions()
	triggerExitCtx, triggerExit := signal.NotifyContext(
		context.Background(), syscall.SIGINT, syscall.SIGUSR1, syscall.SIGTERM,
	)
	defer triggerExit()

	redisClient := redis.NewUniversalClient(o.redisOptions)
	err := waitForRedis(triggerExitCtx, redisClient)
	if err != nil {
		panic(err)
	}

	dsn := postgresOptions.Parse()
	db, err := pgxpool.Connect(triggerExitCtx, dsn)
	if err != nil {
		panic(errors.Tag(err, "cannot talk to postgres"))
	}
	if err = db.Ping(triggerExitCtx); err != nil {
		panic(errors.Tag(err, "cannot talk to postgres"))
	}

	rtm, err := realTime.New(context.Background(), &o.options, db, redisClient)
	if err != nil {
		panic(err)
	}
	go rtm.PeriodicCleanup(triggerExitCtx)

	server := http.Server{
		Addr:    o.address,
		Handler: router.New(rtm, o.options.JWT.RealTime),
	}
	var errServeMux sync.Mutex
	var errServe error
	go func() {
		err2 := server.ListenAndServe()
		errServeMux.Lock()
		errServe = err2
		errServeMux.Unlock()
		triggerExit()
	}()

	<-triggerExitCtx.Done()
	rtm.InitiateGracefulShutdown()
	ctx, done := context.WithTimeout(context.Background(), 15*time.Second)
	defer done()
	pendingShutdown := pendingOperation.TrackOperation(func() error {
		return server.Shutdown(ctx)
	})
	rtm.TriggerGracefulReconnect()
	errClose := pendingShutdown.Wait(context.Background())
	errServeMux.Lock()
	defer errServeMux.Unlock()
	if errServe != nil && errServe != http.ErrServerClosed {
		panic(errServe)
	}
	if errClose != nil {
		panic(errClose)
	}
}
