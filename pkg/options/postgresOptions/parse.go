// Golang port of Overleaf
// Copyright (C) 2022-2023 Jakob Ackermann <das7pad@outlook.com>
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

package postgresOptions

import (
	"net/url"
	"strconv"

	"github.com/das7pad/overleaf-go/pkg/options/env"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

func Parse() string {
	poolSize := env.GetInt("POSTGRES_POOL_SIZE", 25)
	u := sharedTypes.URL{}
	u.Scheme = "postgresql"
	u.User = url.User("postgres")
	u.Host = env.GetString("POSTGRES_HOST", "localhost:5432")
	u.Path = "/postgres"
	//goland:noinspection SpellCheckingInspection
	u.WithQuery(url.Values{
		"sslmode":        {"disable"},
		"pool_max_conns": {strconv.FormatInt(int64(poolSize), 10)},
	})
	return env.GetString("POSTGRES_DSN", u.String())
}
