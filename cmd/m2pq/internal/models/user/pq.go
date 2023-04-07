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

package user

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/netip"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
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
	LastLoginIPField    `bson:"inline"`
	LastNameField       `bson:"inline"`
	LoginCountField     `bson:"inline"`
	MustReconfirmField  `bson:"inline"`
	SignUpDateField     `bson:"inline"`
}

func cleanIP(s string) (string, error) {
	s = strings.TrimPrefix(s, "::ffff:")
	if !strings.ContainsRune(s, ':') {
		return s, nil
	}
	if strings.Count(s, ":") == 7 && s[0] != '[' {
		s = fmt.Sprintf("[%s]:42", s)
	}
	addr, err := netip.ParseAddrPort(s)
	if err != nil {
		return "", errors.Tag(err, "parse ip")
	}
	return addr.Addr().String(), nil
}

func Import(ctx context.Context, db *mongo.Database, rTx, tx pgx.Tx, limit int) error {
	uQuery := bson.M{}
	spQuery := bson.M{}
	{
		var o sharedTypes.UUID
		err := tx.QueryRow(ctx, `
SELECT id
FROM users u
ORDER BY created_at
LIMIT 1
`).Scan(&o)
		if err != nil && err != pgx.ErrNoRows {
			return errors.Tag(err, "get last inserted user")
		}
		if err != pgx.ErrNoRows {
			oldest, err2 := m2pq.UUID2ObjectID(o)
			if err2 != nil {
				return errors.Tag(err2, "decode last insert id")
			}
			uQuery["_id"] = bson.M{
				"$lt": primitive.ObjectID(oldest),
			}
			spQuery["token"] = bson.M{
				"$lt": primitive.ObjectID(oldest).String(),
			}
		}
	}
	uC, err := db.
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

	lwC, err := db.
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

	auditLogs := make(map[sharedTypes.UUID][]AuditLogEntry)
	users := make([][]interface{}, 0, 10)

	i := 0
	for i = 0; uC.Next(ctx) && i < limit; i++ {
		u := ForPQ{}
		if err = uC.Decode(&u); err != nil {
			return errors.Tag(err, "decode user")
		}
		if u.Id == (primitive.ObjectID{}) {
			continue
		}
		uId := m2pq.ObjectID2UUID(u.Id)
		idS := u.Id.Hex()
		log.Printf("users[%d/%d]: %s", i, limit, idS)

		auditLogs[uId] = u.AuditLog

		for idS < lastLW.Token && lwC.Next(ctx) {
			lastLW = learnedWords.ForPQ{}
			if err = lwC.Decode(&lastLW); err != nil {
				return errors.Tag(err, "decode lw")
			}
		}
		if err = lwC.Err(); err != nil {
			return errors.Tag(err, "iter lw cur")
		}

		lw := noLw
		if idS == lastLW.Token {
			lw = lastLW.LearnedWords
		}
		u.LastLoginIP, err = cleanIP(u.LastLoginIP)
		if err != nil {
			return errors.Tag(err, "clean login ip")
		}

		users = append(users, []interface{}{
			u.BetaProgram,           // beta_program
			u.SignUpDate,            // created_at
			nil,                     // deleted_at
			u.EditorConfig,          // editor_config
			u.Email,                 // email
			u.Emails[0].ConfirmedAt, // email_confirmed_at
			u.Emails[0].CreatedAt,   // email_created_at
			u.Epoch,                 // epoch
			u.Features,              // features
			u.FirstName,             // first_name
			uId,                     // id
			u.LastLoggedIn,          // last_login_at
			u.LastLoginIP,           // last_login_ip
			u.LastName,              // last_name
			lw,                      // learned_words
			u.LoginCount,            // login_count
			u.MustReconfirm,         // must_reconfirm
			u.HashedPassword,        // password_hash
		})
		if err != nil {
			return errors.Tag(err, "queue user")
		}
	}
	if err = uC.Err(); err != nil {
		return errors.Tag(err, "iter users")
	}
	_, err = tx.CopyFrom(
		ctx,
		pgx.Identifier{"users"},
		[]string{
			"beta_program", "created_at", "deleted_at", "editor_config",
			"email", "email_confirmed_at", "email_created_at", "epoch",
			"features", "first_name", "id", "last_login_at", "last_login_ip",
			"last_name", "learned_words", "login_count", "must_reconfirm",
			"password_hash",
		},
		pgx.CopyFromRows(users),
	)
	if err != nil {
		return errors.Tag(err, "insert users")
	}
	nAuditLogs := 0
	initiatorMongoIds := make(map[primitive.ObjectID]bool)
	for _, entries := range auditLogs {
		nAuditLogs += len(entries)
		for _, entry := range entries {
			initiatorMongoIds[entry.InitiatorId] = true
		}
	}
	initiatorIds, err := ResolveUsers(ctx, rTx, initiatorMongoIds, nil)
	if err != nil {
		return errors.Tag(err, "resolve audit log users")
	}

	ual := make([][]interface{}, 0, nAuditLogs)
	ids, err := sharedTypes.GenerateUUIDBulk(nAuditLogs)
	if err != nil {
		return errors.Tag(err, "audit log ids")
	}
	for userId, entries := range auditLogs {
		for _, entry := range entries {
			var infoBlob []byte
			infoBlob, err = json.Marshal(entry.Info)
			if err != nil {
				return errors.Tag(err, "serialize audit log")
			}

			entry.IPAddress, err = cleanIP(entry.IPAddress)
			if err != nil {
				return errors.Tag(err, "clean audit log ip")
			}

			ual = append(ual, []interface{}{
				ids.Next(),                      // id
				infoBlob,                        // info
				initiatorIds[entry.InitiatorId], // initiator_id
				entry.IPAddress,                 // ip_address
				entry.Operation,                 // operation
				entry.Timestamp,                 // timestamp
				userId,                          // user_id
			})
		}
	}
	_, err = tx.CopyFrom(
		ctx,
		pgx.Identifier{"user_audit_log"},
		[]string{"id", "info", "initiator_id", "ip_address", "operation", "timestamp", "user_id"},
		pgx.CopyFromRows(ual),
	)
	if err != nil {
		return errors.Tag(err, "insert audit log")
	}
	if i == limit {
		return status.ErrHitLimit
	}
	return nil
}

func ResolveUsers(ctx context.Context, rTx pgx.Tx, ids map[primitive.ObjectID]bool, m map[primitive.ObjectID]pgtype.UUID) (map[primitive.ObjectID]pgtype.UUID, error) {
	idsFlat := make([]sharedTypes.UUID, 0, len(ids))
	for id := range ids {
		idsFlat = append(idsFlat, m2pq.ObjectID2UUID(id))
	}
	r, err := rTx.Query(ctx, `
SELECT id
FROM users
WHERE id = ANY ($1)
`, idsFlat)
	if err != nil {
		return nil, errors.Tag(err, "query users")
	}
	defer r.Close()

	if m == nil {
		m = make(map[primitive.ObjectID]pgtype.UUID, len(ids))
	}
	for r.Next() {
		id := pgtype.UUID{}
		if err = r.Scan(&id); err != nil {
			return nil, errors.Tag(err, "decode user id")
		}
		oId, _ := m2pq.UUID2ObjectID(id.Bytes)
		m[oId] = id
	}
	if err = r.Err(); err != nil {
		return nil, errors.Tag(err, "iter users")
	}

	for id := range ids {
		if _, exists := m[id]; exists {
			continue
		}
		m[id] = pgtype.UUID{}
	}

	return m, nil
}
