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

package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/readpref"

	"github.com/das7pad/document-updater/pkg/managers/documentUpdater"
)

const StartUpTimeout = time.Second * 30

func waitForDataStores(
	ctx context.Context,
	mongoClient *mongo.Client,
	redisClient redis.UniversalClient,
) {
	startUpCtx, doneWithStartupChecks := context.WithTimeout(
		ctx,
		StartUpTimeout,
	)
	ready := make(chan error)
	go func() {
		ready <- waitForMongo(startUpCtx, mongoClient)
	}()
	go func() {
		ready <- waitForRedis(ctx, redisClient)
	}()

	if err := <-ready; err != nil {
		panic(err)
	}
	if err := <-ready; err != nil {
		panic(err)
	}
	close(ready)
	doneWithStartupChecks()
}

func waitForMongo(ctx context.Context, client *mongo.Client) error {
	return client.Ping(ctx, readpref.Primary())
}

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
	mongoClient, err := mongo.Connect(ctx, o.mongoOptions)
	if err != nil {
		panic(err)
	}
	redisClient := redis.NewUniversalClient(o.redisOptions)

	waitForDataStores(ctx, mongoClient, redisClient)

	db := mongoClient.Database(o.dbName)

	// keep reference while building the app
	fmt.Println(db)

	dum, err := documentUpdater.New(o.options, redisClient)
	if err != nil {
		panic(err)
	}

	backgroundTaskCtx, shutdownBackgroundTasks := context.WithCancel(
		context.Background(),
	)
	dum.StartBackgroundTasks(backgroundTaskCtx)

	handler := newHttpController(dum)
	server := http.Server{
		Addr:    o.address,
		Handler: handler.GetRouter(),
	}
	err = server.ListenAndServe()
	shutdownBackgroundTasks()
	if err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}
