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

package chat

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
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

type Message struct {
	Id        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Content   string             `json:"content" bson:"content"`
	Timestamp int64              `json:"timestamp" bson:"timestamp"`
	UserId    primitive.ObjectID `json:"user_id" bson:"user_id"`
	RoomId    primitive.ObjectID `json:"room_id,omitempty" bson:"room_id"`
}

type Room struct {
	Id        primitive.ObjectID `bson:"_id"`
	ProjectId primitive.ObjectID `bson:"project_id"`
}

func Import(ctx context.Context, db *mongo.Database, rTx, tx pgx.Tx, limit int) error {
	uQuery := bson.M{
		"thread_id": bson.M{
			"$exists": false,
		},
	}
	{
		var o sharedTypes.UUID
		err := tx.QueryRow(ctx, `
SELECT project_id
FROM chat_messages
ORDER BY project_id
LIMIT 1
`).Scan(&o)
		if err != nil && err != pgx.ErrNoRows {
			return errors.Tag(err, "get last inserted project id")
		}
		if err != pgx.ErrNoRows {
			oldest, err2 := m2pq.UUID2ObjectID(o)
			if err2 != nil {
				return errors.Tag(err2, "decode last insert id")
			}
			uQuery["project_id"] = bson.M{
				"$lt": primitive.ObjectID(oldest),
			}
		}
	}
	rC, err := db.
		Collection("rooms").
		Find(
			ctx,
			uQuery,
			options.Find().
				SetSort(bson.M{"project_id": -1}).
				SetBatchSize(100),
		)
	if err != nil {
		return errors.Tag(err, "get room cursor")
	}
	defer func() {
		_ = rC.Close(ctx)
	}()

	var mC *mongo.Cursor
	defer func() {
		if mC != nil {
			_ = mC.Close(ctx)
		}
	}()

	lastMsg := Message{}
	lastMsgRoom := "ffffffffffffffffffffffff"

	resolved := make(map[primitive.ObjectID]pgtype.UUID)
	rows := make([][]interface{}, 0, limit)

	i := 0
	for i = 0; rC.Next(ctx) && i < limit; i++ {
		r := Room{}
		if err = rC.Decode(&r); err != nil {
			log.Println(rC.Current.String())
			return errors.Tag(err, "decode room")
		}

		if mC == nil {
			mC, err = db.
				Collection("messages").
				Find(
					ctx,
					bson.M{
						"room_id": bson.M{
							"$lte": r.Id,
						},
					},
					options.Find().
						SetSort(bson.M{"room_id": -1}).
						SetBatchSize(100),
				)
			if err != nil {
				return errors.Tag(err, "get messages cursor")
			}
		}

		roomIdS := r.Id.Hex()
		pId := r.ProjectId
		log.Printf("chat_messages[%d/%d]: %s", i, limit, pId.Hex())

		pending := make([]Message, 0)
		if lastMsgRoom == roomIdS {
			pending = append(pending, lastMsg)
		}

		flush := func() error {
			missing := make(map[primitive.ObjectID]bool)
			for _, msg := range pending {
				if _, exists := resolved[msg.UserId]; !exists {
					missing[msg.UserId] = true
				}
			}

			if len(missing) > 0 {
				resolved, err = user.ResolveUsers(ctx, rTx, missing, resolved)
				if err != nil {
					return errors.Tag(err, "resolve users")
				}
			}

			rows = rows[:0]
			for _, msg := range pending {
				rows = append(rows, []interface{}{
					m2pq.ObjectID2UUID(msg.Id),    // id
					m2pq.ObjectID2UUID(pId),       // project_id
					msg.Content,                   // content
					time.UnixMilli(msg.Timestamp), // created_at
					resolved[msg.UserId],          // user_id
				})
			}
			_, err = tx.CopyFrom(
				ctx,
				pgx.Identifier{"chat_messages"},
				[]string{
					"id", "project_id", "content", "created_at", "user_id",
				},
				pgx.CopyFromRows(rows),
			)
			if err != nil {
				return errors.Tag(err, "insert chat messages")
			}
			pending = pending[:0]
			return nil
		}

		for roomIdS <= lastMsgRoom && mC.Next(ctx) {
			lastMsg = Message{}
			if err = mC.Decode(&lastMsg); err != nil {
				return errors.Tag(err, "decode message")
			}
			lastMsgRoom = lastMsg.RoomId.Hex()

			if lastMsgRoom == roomIdS {
				pending = append(pending, lastMsg)
			}

			if len(pending) >= 100 {
				if err = flush(); err != nil {
					return errors.Tag(err, "flush pending")
				}
			}
		}
		if err = mC.Err(); err != nil {
			return errors.Tag(err, "iter messages cur")
		}
		if len(pending) > 0 {
			if err = flush(); err != nil {
				return errors.Tag(err, "flush pending")
			}
		}
	}
	if err = rC.Err(); err != nil {
		return errors.Tag(err, "iter rooms")
	}
	if i == limit {
		return status.HitLimit
	}
	return nil
}
