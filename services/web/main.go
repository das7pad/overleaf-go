// Golang port of the Overleaf web service
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
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/readpref"

	clsiTypes "github.com/das7pad/overleaf-go/services/clsi/pkg/types"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
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
	o := getOptions()
	ctx := context.Background()
	client, err := mongo.Connect(ctx, o.mongoOptions)
	if err != nil {
		panic(err)
	}
	err = waitForDb(ctx, client)
	if err != nil {
		panic(err)
	}
	db := client.Database(o.dbName)

	redisClient := redis.NewUniversalClient(o.redisOptions)
	err = waitForRedis(ctx, redisClient)
	if err != nil {
		panic(err)
	}

	fmt.Println(o.options)
	wm, err := web.New(o.options, db, redisClient)
	if err != nil {
		panic(err)
	}

	projectId, _ := primitive.ObjectIDFromHex("61228c211d10774f5379c4f9")
	userId, _ := primitive.ObjectIDFromHex("5ec9086c9ca958fbb9ca4b6f")
	rootDocId, _ := primitive.ObjectIDFromHex("61228c211d10774f5379c4fb")

	req := &types.CompileProjectRequest{
		SignedCompileProjectRequestOptions: types.SignedCompileProjectRequestOptions{
			CompileGroup: "priority",
			ProjectId:    projectId,
			UserId:       userId,
			Timeout:      clsiTypes.Timeout(time.Minute),
		},
		AutoCompile: false,
		Draft:       false,
		Compiler:    clsiTypes.PDFLatex,
		CheckMode:   clsiTypes.ErrorCheck,
		ImageName:   "",
		RootDocId:   rootDocId,
	}
	resp := &types.CompileProjectResponse{}

	err = wm.CompileProject(ctx, req, resp)
	fmt.Println(err)
	fmt.Println(resp)

	handler := newHttpController(wm)

	server := http.Server{
		Addr:    o.address,
		Handler: handler.GetRouter(o.corsOptions, o.jwtOptions, redisClient),
	}
	err = server.ListenAndServe()
	if err != nil {
		panic(err)
	}
}
