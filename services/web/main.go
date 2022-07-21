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
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	_ "github.com/lib/pq"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/options/postgresOptions"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web"
	"github.com/das7pad/overleaf-go/services/web/pkg/router"
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
	ctx := context.Background()

	redisClient := redis.NewUniversalClient(o.redisOptions)
	if err := waitForRedis(ctx, redisClient); err != nil {
		panic(err)
	}

	dsn, poolSize := postgresOptions.Parse()
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		panic(errors.Tag(err, "cannot talk to postgres"))
	}
	db.SetMaxIdleConns(poolSize)
	db.SetConnMaxIdleTime(30 * time.Second)
	if err = db.PingContext(ctx); err != nil {
		panic(errors.Tag(err, "cannot talk to postgres"))
	}

	wm, err := web.New(o.options, db, redisClient, "http://"+o.address, nil)
	if err != nil {
		panic(err)
	}

	go func() {
		if !wm.Cron(ctx, false) {
			log.Println("cron failed")
		}
	}()

	if len(os.Args) > 0 && os.Args[len(os.Args)-1] == "cron" {
		if !wm.Cron(ctx, o.dryRunCron) {
			os.Exit(42)
		} else {
			os.Exit(0)
		}
		return
	}

	r := router.New(wm, o.corsOptions)
	if err = http.ListenAndServe(o.address, r); err != nil {
		panic(err)
	}
}
