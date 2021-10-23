// Golang port of Overleaf
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
	"math/rand"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/readpref"

	"github.com/das7pad/overleaf-go/services/real-time/pkg/managers/realTime"
)

func waitForDb(ctx context.Context, client *mongo.Client) error {
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
	// Init the default random source.
	rand.Seed(time.Now().UnixNano())

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

	client, err := mongo.Connect(triggerExitCtx, o.mongoOptions)
	if err != nil {
		panic(err)
	}
	err = waitForDb(triggerExitCtx, client)
	if err != nil {
		panic(err)
	}
	db := client.Database(o.dbName)

	rtm, err := realTime.New(context.Background(), o.options, redisClient, db)
	if err != nil {
		panic(err)
	}
	go rtm.PeriodicCleanup(triggerExitCtx)

	handler := newHttpController(rtm, o.jwtOptions)
	server := http.Server{
		Addr:    o.address,
		Handler: handler.GetRouter(),
	}
	var errServe error
	go func() {
		errServe = server.ListenAndServe()
		triggerExit()
	}()

	<-triggerExitCtx.Done()
	rtm.GracefulShutdown()
	errClose := server.Close()
	if errServe != nil && errServe != http.ErrServerClosed {
		panic(errServe)
	}
	if errClose != nil {
		panic(errClose)
	}
}
