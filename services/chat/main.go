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
	"net/http"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/readpref"

	"github.com/das7pad/overleaf-go/services/chat/pkg/managers/chat"
)

func waitForDb(ctx context.Context, client *mongo.Client) error {
	return client.Ping(ctx, readpref.Primary())
}

func main() {
	address, mongoOptions, dbName := getOptions()
	ctx := context.Background()
	client, err := mongo.Connect(ctx, mongoOptions)
	if err != nil {
		panic(err)
	}
	err = waitForDb(ctx, client)
	if err != nil {
		panic(err)
	}
	db := client.Database(dbName)
	cm := chat.New(db)
	handler := newHttpController(cm)

	server := http.Server{
		Addr:    address,
		Handler: handler.GetRouter(),
	}
	err = server.ListenAndServe()
	if err != nil {
		panic(err)
	}
}
