// Golang port of Overleaf
// Copyright (C) 2022 Jakob Ackermann <das7pad@outlook.com>
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

package utils

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/lib/pq"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/options/postgresOptions"
)

func MustConnectPostgres(timeout time.Duration) *sql.DB {
	ctx, done := context.WithTimeout(context.Background(), timeout)
	defer done()

	dsn := postgresOptions.Parse()
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		panic(errors.Tag(err, "cannot talk to postgres"))
	}
	if err = db.PingContext(ctx); err != nil {
		panic(errors.Tag(err, "cannot talk to postgres"))
	}
	return db
}
