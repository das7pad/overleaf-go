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

package utils

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype/zeronull"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/options/postgresOptions"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

func MustConnectPostgres(ctx context.Context) *pgxpool.Pool {
	ctx, done := context.WithTimeout(ctx, 10*time.Second)
	defer done()

	dsn := postgresOptions.Parse()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		panic(errors.Tag(err, "cannot talk to postgres"))
	}
	db.Config().AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		m := conn.TypeMap()
		zeronull.Register(m)
		m.RegisterDefaultPgType(sharedTypes.UUID{}, "uuid")
		return nil
	}
	if err = db.Ping(ctx); err != nil {
		panic(errors.Tag(err, "cannot talk to postgres"))
	}
	return db
}
