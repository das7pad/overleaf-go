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

package contact

import (
	"context"
	"database/sql"
	"log"

	"github.com/lib/pq"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/das7pad/overleaf-go/cmd/m2pq/internal/models/user"
	"github.com/das7pad/overleaf-go/cmd/m2pq/internal/status"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/m2pq"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type ForPQ struct {
	ContactsField `bson:"inline"`
	UserIdField   `bson:"inline"`
}

func Import(ctx context.Context, db *mongo.Database, rTx, tx *sql.Tx, limit int) error {
	cQuery := bson.M{}
	{
		var o sharedTypes.UUID
		err := tx.QueryRowContext(ctx, `
SELECT b
FROM contacts
ORDER BY b
LIMIT 1
`).Scan(&o)
		if err != nil && err != sql.ErrNoRows {
			return errors.Tag(err, "get last inserted user")
		}
		if err != sql.ErrNoRows {
			lowest, err2 := m2pq.UUID2ObjectID(o)
			if err2 != nil {
				return errors.Tag(err2, "decode last insert id")
			}
			cQuery["user_id"] = bson.M{
				"$lt": primitive.ObjectID(lowest),
			}
		}
	}
	uC, err := db.
		Collection("contacts").
		Find(
			ctx,
			cQuery,
			options.Find().
				SetSort(bson.M{"user_id": -1}).
				SetBatchSize(100),
		)
	if err != nil {
		return errors.Tag(err, "get contacts cursor")
	}
	defer func() {
		_ = uC.Close(ctx)
	}()

	q, err := tx.PrepareContext(
		ctx,
		pq.CopyIn(
			"contacts",
			"a", "b", "connections", "last_touched",
		),
	)
	if err != nil {
		return errors.Tag(err, "prepare insert")
	}
	defer func() {
		_ = q.Close()
	}()
	resolved := make(map[primitive.ObjectID]sql.NullString)
	pending := make([]primitive.ObjectID, 0)

	i := 0
	for i = 0; uC.Next(ctx) && i < limit; i++ {
		u := ForPQ{}
		if err = uC.Decode(&u); err != nil {
			return errors.Tag(err, "decode contact")
		}
		idS := u.UserId.Hex()
		log.Printf("contact[%d/%d]: %s", i, limit, idS)

		pending = pending[:0]
		missing := make(map[primitive.ObjectID]bool)
		if _, exists := resolved[u.UserId]; !exists {
			missing[u.UserId] = true
		}

		var otherId primitive.ObjectID
		for raw := range u.Contacts {
			if idS < raw {
				// We will insert this contact in reverse.
				continue
			}
			otherId, err = primitive.ObjectIDFromHex(raw)
			if err != nil {
				return errors.Tag(err, "parse contact id")
			}
			pending = append(pending, otherId)
			if _, exists := resolved[otherId]; !exists {
				missing[otherId] = true
			}
		}

		if len(missing) > 0 {
			var r map[primitive.ObjectID]sql.NullString
			r, err = user.ResolveUsers(ctx, rTx, missing)
			if err != nil {
				return errors.Tag(err, "resolve users")
			}
			for rId, s := range r {
				resolved[rId] = s
			}
		}

		id := resolved[u.UserId]
		if !id.Valid {
			continue
		}

		for _, other := range pending {
			r := resolved[other]
			if !r.Valid {
				continue
			}
			details := u.Contacts[other.Hex()]
			_, err = q.ExecContext(
				ctx,
				r.String,                   // a
				id.String,                  // b
				details.Connections,        // connections
				details.LastTouched.Time(), // last_touched
			)
			if err != nil {
				return errors.Tag(err, "queue contact")
			}
		}
	}
	if _, err = q.ExecContext(ctx); err != nil {
		return errors.Tag(err, "flush queue")
	}
	if err = q.Close(); err != nil {
		return errors.Tag(err, "finalize statement")
	}
	if err = uC.Err(); err != nil {
		return errors.Tag(err, "close contacts cur")
	}
	if i == limit {
		return status.HitLimit
	}
	return nil
}
