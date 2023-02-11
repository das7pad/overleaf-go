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

package docHistory

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
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
	ProjectId primitive.ObjectID `bson:"project_id"`
	DocId     primitive.ObjectID `bson:"doc_id"`
	Pack      []struct {
		Op []struct {
			Deletion  string `bson:"d" json:"d,omitempty"`
			Insertion string `bson:"i" json:"i,omitempty"`
			Position  int    `bson:"p" json:"p"`
		} `bson:"op"`
		Meta struct {
			StartTS int64              `bson:"start_ts"`
			EndTS   int64              `bson:"end_ts"`
			UserId  primitive.ObjectID `bson:"user_id"`
		}
		Version sharedTypes.Version `bson:"v"`
	} `bson:"pack"`
}

func Import(ctx context.Context, db *mongo.Database, rTx, tx pgx.Tx, limit int) error {
	dhQuery := bson.M{}
	{
		var pId, docId sharedTypes.UUID
		var end time.Time
		err := tx.QueryRow(ctx, `
SELECT p.id, d.id, end_at
FROM doc_history
         INNER JOIN docs d ON doc_history.doc_id = d.id
         INNER JOIN tree_nodes tn ON d.id = tn.id
         INNER JOIN projects p ON tn.project_id = p.id
ORDER BY p.id, d.id, end_at
LIMIT 1
`).Scan(&pId, &docId, &end)
		if err != nil && err != pgx.ErrNoRows {
			return errors.Tag(err, "get last inserted pi")
		}
		if err != pgx.ErrNoRows {
			lowerProjectId, err2 := m2pq.UUID2ObjectID(pId)
			if err2 != nil {
				return errors.Tag(err2, "decode last project id")
			}
			lowerDocId, err2 := m2pq.UUID2ObjectID(docId)
			if err2 != nil {
				return errors.Tag(err2, "decode last doc id")
			}
			dhQuery["$or"] = bson.A{
				bson.M{
					"project_id": bson.M{
						"$lt": primitive.ObjectID(lowerProjectId),
					},
				},
				bson.M{
					"project_id": primitive.ObjectID(lowerProjectId),
					"doc_id": bson.M{
						"$lt": primitive.ObjectID(lowerDocId),
					},
				},
				bson.M{
					"project_id": primitive.ObjectID(lowerProjectId),
					"doc_id":     primitive.ObjectID(lowerDocId),
					"meta.end_ts": bson.M{
						"$lt": end.UnixMilli(),
					},
				},
			}
		}
	}
	dhC, err := db.
		Collection("docHistory").
		Find(
			ctx,
			dhQuery,
			options.Find().
				SetSort(bson.D{
					{Key: "project_id", Value: -1},
					{Key: "doc_id", Value: -1},
					{Key: "meta.ts", Value: -1},
				}).
				SetBatchSize(100),
		)
	if err != nil {
		return errors.Tag(err, "get cursor")
	}
	defer func() {
		_ = dhC.Close(ctx)
	}()

	rows := make([][]interface{}, 0, limit)
	ok := false

	resolved := make(map[primitive.ObjectID]pgtype.UUID)

	i := 0
	for i = 0; dhC.Next(ctx) && i < limit; i++ {
		dh := ForPQ{}
		if err = dhC.Decode(&dh); err != nil {
			return errors.Tag(err, "decode dh")
		}

		docId := m2pq.ObjectID2UUID(dh.DocId)
		projectId := m2pq.ObjectID2UUID(dh.ProjectId)
		err = rTx.QueryRow(ctx, `
SELECT TRUE
FROM docs d
         INNER JOIN tree_nodes tn ON d.id = tn.id
         INNER JOIN projects p ON tn.project_id = p.id
WHERE d.id = $1
  AND p.id = $2
`, docId, projectId).Scan(&ok)
		if err == pgx.ErrNoRows {
			continue
		}
		if err != nil {
			return errors.Tag(err, "check doc exists in project")
		}

		log.Printf("doc_history[%d/%d]: %s", i, limit, dh.DocId.Hex())

		missing := make(map[primitive.ObjectID]bool)
		for _, pack := range dh.Pack {
			if _, exists := resolved[pack.Meta.UserId]; !exists {
				missing[pack.Meta.UserId] = true
			}
		}

		if len(missing) > 0 {
			resolved, err = user.ResolveUsers(ctx, rTx, missing, resolved)
			if err != nil {
				return errors.Tag(err, "resolve users")
			}
		}

		var b *sharedTypes.UUIDBatch
		b, err = sharedTypes.GenerateUUIDBulk(len(dh.Pack))
		if err != nil {
			return errors.Tag(err, "generate insert ids")
		}
		hasBigDelete := false
		for _, pack := range dh.Pack {
			hasBigDelete = false
			for _, component := range pack.Op {
				if len(component.Deletion) > 16 {
					hasBigDelete = true
					break
				}
			}
			var blob []byte
			blob, err = json.Marshal(pack.Op)
			if err != nil {
				return errors.Tag(err, "serialize op")
			}
			rows = append(rows, []interface{}{
				b.Next(),                          // id
				m2pq.ObjectID2UUID(dh.DocId),      // doc_id
				resolved[pack.Meta.UserId],        // user_id
				pack.Version,                      // version
				blob,                              // op
				hasBigDelete,                      // has_big_delete
				time.UnixMilli(pack.Meta.StartTS), // start_at
				time.UnixMilli(pack.Meta.EndTS),   // end_at
			})
		}
	}
	_, err = tx.CopyFrom(
		ctx,
		pgx.Identifier{"doc_history"},
		[]string{
			"id", "doc_id", "user_id", "version", "op", "has_big_delete",
			"start_at", "end_at",
		},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return errors.Tag(err, "insert")
	}

	if i == limit {
		return status.ErrHitLimit
	}
	return nil
}
