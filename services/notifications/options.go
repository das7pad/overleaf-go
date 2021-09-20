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
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/das7pad/overleaf-go/pkg/httpUtils"
	"github.com/das7pad/overleaf-go/pkg/options/corsOptions"
	"github.com/das7pad/overleaf-go/pkg/options/jwtOptions"
	"github.com/das7pad/overleaf-go/pkg/options/listenAddress"
	"github.com/das7pad/overleaf-go/pkg/options/mongoOptions"
)

type notificationsOptions struct {
	address      string
	corsOptions  httpUtils.CORSOptions
	jwtOptions   httpUtils.JWTOptions
	mongoOptions *options.ClientOptions
	dbName       string
}

func getOptions() *notificationsOptions {
	o := &notificationsOptions{}
	o.address = listenAddress.Parse(3042)
	o.corsOptions = corsOptions.Parse()
	o.jwtOptions = jwtOptions.Parse("JWT_NOTIFICATIONS_VERIFY_SECRET")
	o.mongoOptions, o.dbName = mongoOptions.Parse()
	return o
}
