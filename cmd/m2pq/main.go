// Golang port of Overleaf
// Copyright (C) 2022 Jakob Ackermann <das7pad@outlook.com>
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
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/das7pad/overleaf-go/cmd/m2pq/internal/models/user"
	"github.com/das7pad/overleaf-go/cmd/m2pq/internal/mongoOptions"
	"github.com/das7pad/overleaf-go/cmd/m2pq/internal/status"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/options/postgresOptions"
)

func main() {
	timeout := time.Minute
	limit := 1000

	signalCtx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	var mDB *mongo.Database
	{
		ctx, done := context.WithTimeout(signalCtx, timeout)
		defer done()

		mOptions, dbName := mongoOptions.Parse()

		mClient, err := mongo.Connect(ctx, mOptions)
		if err != nil {
			panic(errors.Tag(err, "cannot talk to mongo"))
		}
		if err = mClient.Ping(ctx, nil); err != nil {
			panic(errors.Tag(err, "cannot talk to mongo"))
		}
		done()
		mDB = mClient.Database(dbName)
	}
	var pqDB *sql.DB
	{
		ctx, done := context.WithTimeout(signalCtx, timeout)
		defer done()

		dsn := postgresOptions.Parse()
		db, err := sql.Open("postgres", dsn)
		if err != nil {
			panic(errors.Tag(err, "cannot talk to postgres"))
		}
		if err = db.PingContext(ctx); err != nil {
			panic(errors.Tag(err, "cannot talk to postgres"))
		}
		done()
		pqDB = db
	}

	errCount := 0
	for {
		ctx, done := context.WithTimeout(signalCtx, timeout)
		err := user.Import(ctx, mDB, pqDB, limit)
		done()
		if err == status.HitLimit {
			continue
		}
		if err != nil {
			errCount++
			log.Printf("user import failed: %d: %s", errCount, err)
			if errCount > 100 {
				panic("failed too often")
			}
			continue
		}
		if signalCtx.Err() != nil {
			panic(signalCtx.Err())
		}
		break
	}

	log.Println("Done.")
}
