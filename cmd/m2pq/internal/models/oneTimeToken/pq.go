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

package oneTimeToken

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/das7pad/overleaf-go/cmd/m2pq/internal/status"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/m2pq"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type genericData struct {
	Email  sharedTypes.Email `bson:"email"`
	UserId interface{}       `bson:"user_id"`
}

func (g *genericData) GetUserId() (primitive.ObjectID, error) {
	switch id := g.UserId.(type) {
	case string:
		return primitive.ObjectIDFromHex(id)
	case primitive.ObjectID:
		return id, nil
	default:
		return primitive.ObjectID{}, &errors.InvalidStateError{
			Msg: fmt.Sprintf("unexpected data.user_id: %T %q", id, id),
		}
	}
}

type ForPQ struct {
	CreatedAtField `bson:"inline"`
	Data           genericData `bson:"data"`
	ExpiresAtField `bson:"inline"`
	TokenField     `bson:"inline"`
	UseField       `bson:"inline"`
	UsedAtField    `bson:"inline"`
}

func Import(ctx context.Context, db *mongo.Database, _, tx pgx.Tx, limit int) error {
	ottQuery := bson.M{}
	{
		var oldest time.Time
		err := tx.QueryRow(ctx, `
SELECT created_at
FROM one_time_tokens
ORDER BY created_at
LIMIT 1
`).Scan(&oldest)
		if err != nil && err != pgx.ErrNoRows {
			return errors.Tag(err, "get last inserted pi")
		}
		if err != pgx.ErrNoRows {
			ottQuery["createdAt"] = bson.M{
				"$lt": oldest,
			}
		}
	}
	ottC, err := db.
		Collection("tokens").
		Find(
			ctx,
			ottQuery,
			options.Find().
				SetSort(bson.M{"createdAt": -1}).
				SetBatchSize(100),
		)
	if err != nil {
		return errors.Tag(err, "get cursor")
	}
	defer func() {
		_ = ottC.Close(ctx)
	}()

	rows := make([][]interface{}, 0, limit)
	i := 0
	for i = 0; ottC.Next(ctx) && i < limit; i++ {
		ott := ForPQ{}
		if err = ottC.Decode(&ott); err != nil {
			return errors.Tag(err, "decode ott")
		}
		log.Printf("one_time_token[%d/%d]: %s", i, limit, ott.CreatedAt)

		var userId primitive.ObjectID
		if userId, err = ott.Data.GetUserId(); err != nil {
			return errors.Tag(err, "decode user id")
		}
		rows = append(rows, []interface{}{
			ott.CreatedAt,              // created_at
			ott.Data.Email,             // email
			ott.ExpiresAt,              // expires_at
			ott.Token,                  // token
			ott.Use,                    // use
			ott.UsedAt,                 // used_at
			m2pq.ObjectID2UUID(userId), // user_id
		})
	}
	_, err = tx.CopyFrom(
		ctx,
		pgx.Identifier{"one_time_tokens"},
		[]string{"created_at", "email", "expires_at", "token", "use", "used_at", "user_id"},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return errors.Tag(err, "insert one time tokens")
	}

	if i == limit {
		return status.ErrHitLimit
	}
	return nil
}
