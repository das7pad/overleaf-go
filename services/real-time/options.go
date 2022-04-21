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

package main

import (
	"github.com/go-redis/redis/v8"

	"github.com/das7pad/overleaf-go/pkg/options/jwtOptions"
	"github.com/das7pad/overleaf-go/pkg/options/listenAddress"
	"github.com/das7pad/overleaf-go/pkg/options/redisOptions"
	"github.com/das7pad/overleaf-go/pkg/options/utils"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

type realTimeOptions struct {
	address      string
	jwtOptions   jwtOptions.JWTOptions
	redisOptions *redis.UniversalOptions
	options      *types.Options
}

func getOptions() *realTimeOptions {
	o := &realTimeOptions{}
	utils.ParseJSONFromEnv("OPTIONS", &o.options)
	o.address = listenAddress.Parse(3026)
	o.jwtOptions = jwtOptions.Parse("JWT_REAL_TIME_VERIFY_SECRET")
	o.redisOptions = redisOptions.Parse()
	return o
}
