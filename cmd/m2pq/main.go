// Golang port of Overleaf
// Copyright (C) 2022-2024 Jakob Ackermann <das7pad@outlook.com>
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
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/das7pad/overleaf-go/cmd/back-fill-project-last-updated/pkg/backFillProjectLastUpdated"
	"github.com/das7pad/overleaf-go/cmd/m2pq/internal/models/chat"
	"github.com/das7pad/overleaf-go/cmd/m2pq/internal/models/contact"
	"github.com/das7pad/overleaf-go/cmd/m2pq/internal/models/docHistory"
	"github.com/das7pad/overleaf-go/cmd/m2pq/internal/models/notification"
	"github.com/das7pad/overleaf-go/cmd/m2pq/internal/models/oneTimeToken"
	"github.com/das7pad/overleaf-go/cmd/m2pq/internal/models/project"
	"github.com/das7pad/overleaf-go/cmd/m2pq/internal/models/projectInvite"
	"github.com/das7pad/overleaf-go/cmd/m2pq/internal/models/tag"
	"github.com/das7pad/overleaf-go/cmd/m2pq/internal/models/user"
	"github.com/das7pad/overleaf-go/cmd/m2pq/internal/mongoOptions"
	"github.com/das7pad/overleaf-go/cmd/m2pq/internal/status"
	"github.com/das7pad/overleaf-go/cmd/pkg/utils"
	"github.com/das7pad/overleaf-go/pkg/errors"
)

type importer struct {
	name string
	fn   func(ctx context.Context, db *mongo.Database, rTx, tx pgx.Tx, limit int) error
}

func main() {
	timeout := time.Minute
	limit := 100

	signalCtx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	var mDB *mongo.Database
	{
		ctx, done := context.WithTimeout(signalCtx, timeout)
		defer done()

		mOptions, dbName := mongoOptions.Parse()

		mClient, err := mongo.Connect(ctx, mOptions)
		if err != nil {
			panic(errors.Tag(err, "connect to mongo"))
		}
		if err = mClient.Ping(ctx, nil); err != nil {
			panic(errors.Tag(err, "ping to mongo"))
		}
		done()
		mDB = mClient.Database(dbName)
	}
	var pqDB *pgxpool.Pool
	{
		ctx, done := context.WithTimeout(signalCtx, timeout)
		pqDB = utils.MustConnectPostgres(ctx)
		done()
	}

	queue := []importer{
		{name: "users", fn: user.Import},
		{name: "contacts", fn: contact.Import},
		{name: "projects", fn: project.Import},
		{name: "tags", fn: tag.Import},
		{name: "project_invites", fn: projectInvite.Import},
		{name: "one_time_tokens", fn: oneTimeToken.Import},
		{name: "notifications", fn: notification.Import},
		{name: "doc_history", fn: docHistory.Import},
		{name: "chat_messages", fn: chat.Import},
	}
	var rTx pgx.Tx
	{
		var err error
		rTx, err = pqDB.Begin(signalCtx)
		if err != nil {
			panic(errors.Tag(err, "open read tx"))
		}
	}
	defer func() { _ = rTx.Rollback(signalCtx) }()

	for _, task := range queue {
		name := task.name
		errCount := 0
		log.Printf("%s import start", name)
		for {
			if signalCtx.Err() != nil {
				panic(signalCtx.Err())
			}

			ctx, done := context.WithTimeout(signalCtx, timeout)
			tx, err := pqDB.Begin(ctx)
			if err != nil {
				panic(errors.Tag(err, "start tx"))
			}
			err = task.fn(ctx, mDB, rTx, tx, limit)
			if err == nil || err == status.ErrHitLimit {
				if err2 := tx.Commit(signalCtx); err2 != nil {
					panic(errors.Tag(err2, "commit tx"))
				}
				done()
				if err == status.ErrHitLimit {
					errCount = 0
					continue
				}
				break
			}
			errCount++
			log.Printf("%s import failed: %d: %s", name, errCount, err)
			if err = tx.Rollback(signalCtx); err != nil {
				panic(errors.Tag(err, "rollback tx"))
			}
			if errCount > 100 {
				panic("failed too often")
			}
			done()
			continue
		}
		log.Printf("%s import done", name)
	}

	if err := backFillProjectLastUpdated.Run(signalCtx, pqDB); err != nil {
		panic(err)
	}

	log.Println("Done.")
}
