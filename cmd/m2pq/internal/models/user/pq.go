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

package user

import (
	"context"
	"database/sql"
	"log"
	"net/netip"
	"strings"

	"github.com/lib/pq"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/das7pad/overleaf-go/cmd/m2pq/internal/models/learnedWords"
	"github.com/das7pad/overleaf-go/cmd/m2pq/internal/status"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/m2pq"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type ForPQ struct {
	AuditLogField       `bson:"inline"`
	BetaProgramField    `bson:"inline"`
	EditorConfigField   `bson:"inline"`
	EmailField          `bson:"inline"`
	EmailsField         `bson:"inline"`
	EpochField          `bson:"inline"`
	FeaturesField       `bson:"inline"`
	FirstNameField      `bson:"inline"`
	IdField             `bson:"inline"`
	HashedPasswordField `bson:"inline"`
	LastLoggedInField   `bson:"inline"`
	LastLoginIpField    `bson:"inline"`
	LastNameField       `bson:"inline"`
	LoginCountField     `bson:"inline"`
	MustReconfirmField  `bson:"inline"`
	SignUpDateField     `bson:"inline"`
}

func Import(ctx context.Context, mDB *mongo.Database, pqDB *sql.DB, limit int) error {
	uQuery := bson.M{}
	spQuery := bson.M{}
	{
		var o sharedTypes.UUID
		err := pqDB.QueryRowContext(ctx, `
SELECT id
FROM users u
ORDER BY signup_date
LIMIT 1
`).Scan(&o)
		if err != nil && err != sql.ErrNoRows {
			return errors.Tag(err, "cannot get last inserted user")
		}
		if err != sql.ErrNoRows {
			oldest, err2 := m2pq.UUID2ObjectID(o)
			if err2 != nil {
				return errors.Tag(err2, "cannot decode last insert id")
			}
			uQuery["_id"] = bson.M{
				"$lt": primitive.ObjectID(oldest),
			}
			uQuery["token"] = bson.M{
				"$lt": primitive.ObjectID(oldest).String(),
			}
		}
	}
	uC, err := mDB.
		Collection("users").
		Find(
			ctx,
			uQuery,
			options.Find().
				SetSort(bson.M{"_id": -1}).
				SetBatchSize(100),
		)
	if err != nil {
		return errors.Tag(err, "get user cursor")
	}
	defer func() {
		_ = uC.Close(ctx)
	}()

	lwC, err := mDB.
		Collection("spellingPreferences").
		Find(
			ctx,
			spQuery,
			options.Find().
				SetSort(bson.M{"token": -1}).
				SetBatchSize(100),
		)
	if err != nil {
		return errors.Tag(err, "get lw cursor")
	}
	defer func() {
		_ = lwC.Close(ctx)
	}()

	lastLW := learnedWords.ForPQ{
		UserIdField: learnedWords.UserIdField{
			Token: "ffffffffffffffffffffffff",
		},
	}
	noLw := make([]string, 0)

	ok := false
	tx, err := pqDB.BeginTx(ctx, nil)
	if err != nil {
		return errors.Tag(err, "start tx")
	}
	defer func() {
		if !ok {
			_ = tx.Rollback()
		}
	}()
	q, err := tx.PrepareContext(
		ctx,
		pq.CopyIn(
			"users",
			"beta_program", "deleted_at", "editor_config", "email", "email_confirmed_at", "email_created_at", "epoch", "features", "first_name", "id", "last_login_at", "last_login_ip", "last_name", "learned_words", "login_count", "must_reconfirm", "password_hash", "signup_date",
		),
	)
	if err != nil {
		return errors.Tag(err, "prepare insert")
	}
	defer func() {
		if !ok && q != nil {
			_ = q.Close()
		}
	}()

	i := 0
	for i = 0; uC.Next(ctx) && i < limit; i++ {
		u := ForPQ{}
		if err = uC.Decode(&u); err != nil {
			return errors.Tag(err, "cannot decode user")
		}
		if u.Id == (primitive.ObjectID{}) {
			continue
		}
		idS := u.Id.Hex()
		log.Printf("user[%d/%d]: %s", i, limit, idS)
		for idS < lastLW.Token && lwC.Next(ctx) {
			if err = lwC.Decode(&lastLW); err != nil {
				return errors.Tag(err, "cannot decode lw")
			}
		}
		if err = lwC.Err(); err != nil {
			return errors.Tag(err, "cannot iter lw cur")
		}

		lw := noLw
		if idS == lastLW.Token {
			lw = lastLW.LearnedWords
		}
		if strings.ContainsRune(u.LastLoginIp, ':') {
			addr, err2 := netip.ParseAddrPort(u.LastLoginIp)
			if err2 != nil {
				return errors.Tag(err2, "parse login ip")
			}
			u.LastLoginIp = addr.Addr().String()
		}

		_, err = q.ExecContext(
			ctx,
			u.BetaProgram,            // beta_program
			nil,                      // deleted_at
			u.EditorConfig,           // editor_config
			u.Email,                  // email
			u.Emails[0].ConfirmedAt,  // email_confirmed_at
			u.Emails[0].CreatedAt,    // email_created_at
			u.Epoch,                  // epoch
			u.Features,               // features
			u.FirstName,              // first_name
			m2pq.ObjectID2UUID(u.Id), // id
			u.LastLoggedIn,           // last_login_at
			u.LastLoginIp,            // last_login_ip
			u.LastName,               // last_name
			pq.Array(lw),             // learned_words
			u.LoginCount,             // login_count
			u.MustReconfirm,          // must_reconfirm
			u.HashedPassword,         // password_hash
			u.SignUpDate,             // signup_date
		)
		if err != nil {
			return errors.Tag(err, "queue user")
		}
	}
	if _, err = q.ExecContext(ctx); err != nil {
		return errors.Tag(err, "flush queue")
	}
	if err = q.Close(); err != nil {
		return errors.Tag(err, "finalize statement")
	}
	if err = tx.Commit(); err != nil {
		return errors.Tag(err, "commit tx")
	}
	ok = true
	if err = uC.Err(); err != nil {
		return errors.Tag(err, "cannot iter user cur")
	}
	if i == limit {
		return status.HitLimit
	}
	return nil
}
