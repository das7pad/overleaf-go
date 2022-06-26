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

package projectInvite

import (
	"context"
	"database/sql"
	"log"

	"github.com/lib/pq"
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
	CreatedAtField      `bson:"inline"`
	EmailField          `bson:"inline"`
	ExpiresAtField      `bson:"inline"`
	IdField             `bson:"inline"`
	PrivilegeLevelField `bson:"inline"`
	ProjectIdField      `bson:"inline"`
	SendingUserIdField  `bson:"inline"`
	TokenField          `bson:"inline"`
}

func Import(ctx context.Context, db *mongo.Database, _, tx *sql.Tx, limit int) error {
	piQuery := bson.M{}
	{
		var o sharedTypes.UUID
		err := tx.QueryRowContext(ctx, `
SELECT id
FROM project_invites
ORDER BY created_at
LIMIT 1
`).Scan(&o)
		if err != nil && err != sql.ErrNoRows {
			return errors.Tag(err, "get last inserted pi")
		}
		if err != sql.ErrNoRows {
			oldest, err2 := m2pq.UUID2ObjectID(o)
			if err2 != nil {
				return errors.Tag(err2, "decode last insert id")
			}
			piQuery["_id"] = bson.M{
				"$lt": primitive.ObjectID(oldest),
			}
		}
	}
	piC, err := db.
		Collection("projectInvites").
		Find(
			ctx,
			piQuery,
			options.Find().
				SetSort(bson.M{"_id": -1}).
				SetBatchSize(100),
		)
	if err != nil {
		return errors.Tag(err, "get cursor")
	}
	defer func() {
		_ = piC.Close(ctx)
	}()

	q, err := tx.PrepareContext(
		ctx,
		pq.CopyIn(
			"project_invites",
			"created_at", "email", "expires_at", "id", "privilege_level", "project_id", "sending_user_id", "token",
		),
	)
	if err != nil {
		return errors.Tag(err, "prepare insert")
	}
	defer func() {
		_ = q.Close()
	}()

	i := 0
	for i = 0; piC.Next(ctx) && i < limit; i++ {
		pi := ForPQ{}
		if err = piC.Decode(&pi); err != nil {
			return errors.Tag(err, "decode pi")
		}
		log.Printf("project_invite[%d/%d]: %s", i, limit, pi.Id.Hex())

		_, err = q.ExecContext(
			ctx,
			pi.CreatedAt,                         //  created_at
			pi.Email,                             //  email
			pi.Expires,                           //  expires_at
			m2pq.ObjectID2UUID(pi.Id),            //  id
			pi.PrivilegeLevel,                    //  privilege_level
			m2pq.ObjectID2UUID(pi.ProjectId),     //  project_id
			m2pq.ObjectID2UUID(pi.SendingUserId), //  sending_user_id
			pi.Token,                             //  token
		)
		if err != nil {
			return errors.Tag(err, "queue pi")
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
