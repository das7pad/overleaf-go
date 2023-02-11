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

package tag

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
	IdField         `bson:"inline"`
	NameField       `bson:"inline"`
	ProjectIdsField `bson:"inline"`
	UserIdField     `bson:"inline"`
}

func Import(ctx context.Context, db *mongo.Database, _, tx pgx.Tx, limit int) error {
	tQuery := bson.M{}
	{
		var o sharedTypes.UUID
		err := tx.QueryRow(ctx, `
SELECT user_id
FROM tags
ORDER BY user_id
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
			tQuery["user_id"] = bson.M{
				"$lt": primitive.ObjectID(lowest),
			}
		}
	}
	tC, err := db.
		Collection("tags").
		Find(
			ctx,
			tQuery,
			options.Find().
				SetSort(bson.M{"user_id": -1}).
				SetBatchSize(int32(limit)).
				SetLimit(int64(limit)),
		)
	if err != nil {
		return errors.Tag(err, "get contacts cursor")
	}
	tags := make([]ForPQ, limit)
	if err = tC.All(ctx, &tags); err != nil {
		return errors.Tag(err, "fetch all tags")
	}

	// Part 1: user <-> tag with name
	tagRows := make([][]interface{}, 0, len(tags))
	var userId primitive.ObjectID
	for i, t := range tags {
		log.Printf("tags[%d/%d]: tags: %s", i, limit, t.Id.Hex())

		userId, err = primitive.ObjectIDFromHex(t.UserId)
		if err != nil {
			return errors.Tag(err, "parse user id")
		}
		tagRows = append(tagRows, []interface{}{
			m2pq.ObjectID2UUID(t.Id), t.Name, m2pq.ObjectID2UUID(userId),
		})
	}
	_, err = tx.CopyFrom(
		ctx,
		pgx.Identifier{"tags"},
		[]string{"id", "name", "user_id"},
		pgx.CopyFromRows(tagRows),
	)
	if err != nil {
		return errors.Tag(err, "insert tags")
	}

	// Part 2: tag <-> project
	tagEntryRows := make([][]interface{}, 0, len(tags))
	for _, t := range tags {
		for _, projectId := range t.ProjectIds {
			tagEntryRows = append(tagEntryRows, []interface{}{
				m2pq.ObjectID2UUID(projectId), m2pq.ObjectID2UUID(t.Id),
			})
		}
	}
	_, err = tx.CopyFrom(
		ctx,
		pgx.Identifier{"tag_entries"},
		[]string{"project_id", "tag_id"},
		pgx.CopyFromRows(tagEntryRows),
	)
	if err != nil {
		return errors.Tag(err, "insert tags")
	}

	if len(tags) >= limit {
		return status.ErrHitLimit
	}
	return nil
}
