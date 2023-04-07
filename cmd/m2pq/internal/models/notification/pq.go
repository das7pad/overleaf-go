// Golang port of Overleaf
// Copyright (C) 2021-2023 Jakob Ackermann <das7pad@outlook.com>
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

package notification

import (
	"context"
	"log"

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

type ForPQ struct {
	IdField          `bson:"inline"`
	KeyField         `bson:"inline"`
	UserIdField      `bson:"inline"`
	ExpiresField     `bson:"inline"`
	TemplateKeyField `bson:"inline"`
	MessageOptsField `bson:"inline"`
}

func Import(ctx context.Context, db *mongo.Database, _, tx pgx.Tx, limit int) error {
	ottQuery := bson.M{}
	{
		var o sharedTypes.UUID
		err := tx.QueryRow(ctx, `
SELECT id
FROM notifications
ORDER BY id
LIMIT 1
`).Scan(&o)
		if err != nil && err != pgx.ErrNoRows {
			return errors.Tag(err, "get last inserted user")
		}
		if err != pgx.ErrNoRows {
			lowest, err2 := m2pq.UUID2ObjectID(o)
			if err2 != nil {
				return errors.Tag(err2, "decode last insert id")
			}
			ottQuery["_id"] = bson.M{
				"$lt": primitive.ObjectID(lowest),
			}
		}
	}
	nC, err := db.
		Collection("notifications").
		Find(
			ctx,
			ottQuery,
			options.Find().
				SetSort(bson.M{"_id": -1}).
				SetBatchSize(100),
		)
	if err != nil {
		return errors.Tag(err, "get cursor")
	}
	defer func() {
		_ = nC.Close(ctx)
	}()

	rows := make([][]interface{}, 0, limit)
	i := 0
	for i = 0; nC.Next(ctx) && i < limit; i++ {
		n := ForPQ{}
		if err = nC.Decode(&n); err != nil {
			return errors.Tag(err, "decode notification")
		}
		log.Printf("notifications[%d/%d]: %s", i, limit, n.Id.Hex())

		rows = append(rows, []interface{}{
			n.Expires,                    // expires_at
			m2pq.ObjectID2UUID(n.Id),     // id
			n.Key,                        // key
			n.MessageOptions,             // message_options
			n.TemplateKey,                // template_key
			m2pq.ObjectID2UUID(n.UserId), // user_id
		})
	}
	_, err = tx.CopyFrom(
		ctx,
		pgx.Identifier{"notifications"},
		[]string{
			"expires_at", "id", "key", "message_options", "template_key",
			"user_id",
		},
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
