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
	"github.com/das7pad/overleaf-go/pkg/options/utils"
	"github.com/das7pad/overleaf-go/services/spelling/pkg/types"
)

type spellingOptions struct {
	address      string
	corsOptions  httpUtils.CORSOptions
	jwtOptions   jwtOptions.JWTOptions
	mongoOptions *options.ClientOptions
	dbName       string
	options      *types.Options
}

func getOptions() *spellingOptions {
	o := &spellingOptions{}
	utils.ParseJSONFromEnv("OPTIONS", &o.options)
	o.address = listenAddress.Parse(3005)
	o.corsOptions = corsOptions.Parse()
	o.jwtOptions = jwtOptions.Parse("JWT_SPELLING_VERIFY_SECRET")
	o.mongoOptions, o.dbName = mongoOptions.Parse()
	return o
}
