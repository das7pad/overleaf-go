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
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v4/pgxpool"
	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/httpUtils"
	"github.com/das7pad/overleaf-go/pkg/options/postgresOptions"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater"
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
	triggerExitCtx, triggerExit := signal.NotifyContext(
		context.Background(), syscall.SIGINT, syscall.SIGUSR1, syscall.SIGTERM,
	)
	defer triggerExit()
	o := getOptions()
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

	dum, err := documentUpdater.New(&o.options, db, redisClient)
	if err != nil {
		panic(err)
	}

	server := http.Server{
		Addr:    o.address,
		Handler: httpUtils.NewRouter(&httpUtils.RouterOptions{}),
	}
	eg, ctx := errgroup.WithContext(triggerExitCtx)
	eg.Go(func() error {
		dum.ProcessDocumentUpdates(ctx)
		return nil
	})
	eg.Go(func() error {
		return server.ListenAndServe()
	})
	eg.Go(func() error {
		<-ctx.Done()
		return server.Shutdown(context.Background())
	})
	err = eg.Wait()
	if err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}
