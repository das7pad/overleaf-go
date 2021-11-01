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
	"github.com/go-redis/redis/v8"

	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/das7pad/overleaf-go/pkg/httpUtils"
	"github.com/das7pad/overleaf-go/pkg/options/corsOptions"
	"github.com/das7pad/overleaf-go/pkg/options/listenAddress"
	"github.com/das7pad/overleaf-go/pkg/options/mongoOptions"
	"github.com/das7pad/overleaf-go/pkg/options/redisOptions"
	"github.com/das7pad/overleaf-go/pkg/options/utils"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type webOptions struct {
	address      string
	corsOptions  httpUtils.CORSOptions
	mongoOptions *options.ClientOptions
	dbName       string
	redisOptions *redis.UniversalOptions
	options      *types.Options
}

func getOptions() *webOptions {
	o := &webOptions{}
	utils.ParseJSONFromEnv("OPTIONS", &o.options)
	o.address = listenAddress.Parse(4000)
	o.corsOptions = corsOptions.Parse()
	o.options.JWT.Compile.FillFromEnv("JWT_WEB_VERIFY_SECRET")
	o.options.JWT.LoggedInUser.FillFromEnv("JWT_WEB_VERIFY_SECRET")
	o.options.JWT.Notifications.FillFromEnv("JWT_NOTIFICATIONS_VERIFY_SECRET")
	o.options.JWT.Spelling.FillFromEnv("JWT_SPELLING_VERIFY_SECRET")
	o.options.JWT.RealTime.FillFromEnv("JWT_REAL_TIME_VERIFY_SECRET")
	o.options.SessionCookie.FillFromEnv("SESSION_SECRET")
	o.mongoOptions, o.dbName = mongoOptions.Parse()
	o.redisOptions = redisOptions.Parse()
	return o
}
