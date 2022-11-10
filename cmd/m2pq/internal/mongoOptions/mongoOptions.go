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

package mongoOptions

import (
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/options/env"
)

func Parse() (*options.ClientOptions, string) {
	mongoConnectionString := env.GetString(
		"MONGO_CONNECTION_STRING",
		fmt.Sprintf(
			"mongodb://%s/sharelatex",
			env.GetString("MONGO_HOST", "localhost"),
		),
	)
	mongoOptions := options.Client()
	mongoOptions.ApplyURI(mongoConnectionString)
	mongoOptions.SetAppName(env.GetString("SERVICE_NAME", ""))
	mongoOptions.SetMaxPoolSize(
		uint64(env.GetInt("MONGO_POOL_SIZE", 10)),
	)
	mongoOptions.SetSocketTimeout(
		env.GetDuration("MONGO_SOCKET_TIMEOUT", 30*time.Second),
	)
	mongoOptions.SetServerSelectionTimeout(env.GetDuration(
		"MONGO_SERVER_SELECTION_TIMEOUT",
		60*time.Second,
	))

	cs, err := connstring.Parse(mongoConnectionString)
	if err != nil {
		panic(errors.Tag(err, "parse connection string"))
	}
	dbName := cs.Database

	return mongoOptions, dbName
}
