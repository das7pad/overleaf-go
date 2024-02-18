// Golang port of Overleaf
// Copyright (C) 2022-2024 Jakob Ackermann <das7pad@outlook.com>
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
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/options/postgresOptions"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

func MustConnectPostgres(ctx context.Context) *pgxpool.Pool {
	ctx, done := context.WithTimeout(ctx, 10*time.Second)
	defer done()

	var cfg *pgxpool.Config
	{
		var err error
		cfg, err = pgxpool.ParseConfig(postgresOptions.Parse())
		if err != nil {
			panic(errors.Tag(err, "parse postgres DSN"))
		}
	}
	var extraTypes []*pgtype.Type
	var extraTypesMu sync.Mutex
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		m := conn.TypeMap()
		extraTypesMu.Lock()
		if len(extraTypes) == 0 {
			var err error
			extraTypes, err = registerEnumTypes(ctx, conn)
			if err != nil {
				extraTypesMu.Unlock()
				return errors.Tag(err, "register enum types")
			}
			extraTypes = append(extraTypes, &pgtype.Type{
				Codec: sharedTypes.UUIDCodec{},
				Name:  "uuid",
				OID:   pgtype.UUIDOID,
			})
			extraTypes = append(extraTypes, &pgtype.Type{
				Codec: sharedTypes.UUIDsCodec{},
				Name:  "_uuid",
				OID:   pgtype.UUIDArrayOID,
			})
		}
		extraTypesMu.Unlock()
		for _, t := range extraTypes {
			m.RegisterType(t)
		}
		return nil
	}
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		panic(errors.Tag(err, "connect to postgres"))
	}
	if err = db.Ping(ctx); err != nil {
		panic(errors.Tag(err, "ping to postgres"))
	}
	return db
}

func registerEnumTypes(ctx context.Context, conn *pgx.Conn) ([]*pgtype.Type, error) {
	//goland:noinspection SpellCheckingInspection
	rows, err := conn.Query(
		ctx, `SELECT oid, typname FROM pg_type WHERE typtype = 'e'`,
	)
	if err != nil {
		return nil, errors.Tag(err, "list enum types")
	}
	defer rows.Close()
	tt := make([]*pgtype.Type, 0, 10)
	for rows.Next() {
		t := pgtype.Type{Codec: &pgtype.EnumCodec{}}
		if err = rows.Scan(&t.OID, &t.Name); err != nil {
			return nil, errors.Tag(err, "scan enum type")
		}
		tt = append(tt, &t)
	}
	if err = rows.Err(); err != nil {
		return nil, errors.Tag(err, "iter enum types")
	}
	return tt, nil
}
