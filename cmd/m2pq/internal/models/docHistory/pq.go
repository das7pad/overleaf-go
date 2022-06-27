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

package docHistory

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"time"

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
	ProjectId primitive.ObjectID `bson:"project_id"`
	DocId     primitive.ObjectID `bson:"doc_id"`
	Pack      []struct {
		Op []struct {
			Deletion  string `bson:"d" json:"d,omitempty"`
			Insertion string `bson:"i" json:"i,omitempty"`
			Position  int    `bson:"p" json:"p"`
		} `bson:"op"`
		Meta struct {
			StartTs int64              `bson:"start_ts"`
			EndTs   int64              `bson:"end_ts"`
			UserId  primitive.ObjectID `bson:"user_id"`
		}
		Version sharedTypes.Version `bson:"v"`
	} `bson:"pack"`
}

func Import(ctx context.Context, db *mongo.Database, rTx, tx *sql.Tx, limit int) error {
	dhQuery := bson.M{}
	{
		var pId, docId sharedTypes.UUID
		var end time.Time
		err := tx.QueryRowContext(ctx, `
SELECT p.id, d.id, end_at
FROM doc_history
         INNER JOIN docs d ON doc_history.doc_id = d.id
         INNER JOIN tree_nodes tn ON d.id = tn.id
         INNER JOIN projects p ON tn.project_id = p.id
ORDER BY p.id, d.id, end_at
LIMIT 1
`).Scan(&pId, &docId, &end)
		if err != nil && err != sql.ErrNoRows {
			return errors.Tag(err, "get last inserted pi")
		}
		if err != sql.ErrNoRows {
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

	q, err := tx.PrepareContext(
		ctx,
		pq.CopyIn(
			"doc_history",
			"id", "doc_id", "user_id", "version", "op",
			"has_big_delete", "start_at", "end_at",
		),
	)
	if err != nil {
		return errors.Tag(err, "prepare insert")
	}
	defer func() {
		_ = q.Close()
	}()

	checkStmt, err := rTx.PrepareContext(ctx, `
SELECT TRUE
FROM docs d
         INNER JOIN tree_nodes tn ON d.id = tn.id
         INNER JOIN projects p ON tn.project_id = p.id
WHERE d.id = $1
  AND p.id = $2
`)
	if err != nil {
		return errors.Tag(err, "prepare check statement")
	}
	ok := false

	resolved := make(map[primitive.ObjectID]sql.NullString)

	i := 0
	for i = 0; dhC.Next(ctx) && i < limit; i++ {
		dh := ForPQ{}
		if err = dhC.Decode(&dh); err != nil {
			return errors.Tag(err, "decode dh")
		}

		docId := m2pq.ObjectID2UUID(dh.DocId)
		projectId := m2pq.ObjectID2UUID(dh.ProjectId)
		err = checkStmt.QueryRowContext(ctx, docId, projectId).Scan(&ok)
		if err == sql.ErrNoRows {
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
			var r map[primitive.ObjectID]sql.NullString
			r, err = user.ResolveUsers(ctx, rTx, missing)
			if err != nil {
				return errors.Tag(err, "resolve users")
			}
			for id, s := range r {
				resolved[id] = s
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
			_, err = q.ExecContext(
				ctx,
				b.Next(),                          // id
				m2pq.ObjectID2UUID(dh.DocId),      // doc_id
				resolved[pack.Meta.UserId],        // user_id
				pack.Version,                      // version
				string(blob),                      // op
				hasBigDelete,                      // has_big_delete
				time.UnixMilli(pack.Meta.StartTs), // start_at
				time.UnixMilli(pack.Meta.EndTs),   // end_at
			)
			if err != nil {
				return errors.Tag(err, "queue")
			}
		}
	}
	if _, err = q.ExecContext(ctx); err != nil {
		return errors.Tag(err, "flush queue")
	}
	if err = q.Close(); err != nil {
		return errors.Tag(err, "finalize statement")
	}

	if i == limit {
		return status.HitLimit
	}
	return nil
}
