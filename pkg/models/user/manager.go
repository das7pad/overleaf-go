// Golang port of Overleaf
// Copyright (C) 2021-2022 Jakob Ackermann <das7pad@outlook.com>
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
	"encoding/json"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/lib/pq"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/oneTimeToken"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Manager interface {
	CreateUser(ctx context.Context, u *ForCreation) error
	SoftDelete(ctx context.Context, userId sharedTypes.UUID, ip string) error
	HardDelete(ctx context.Context, userId sharedTypes.UUID) error
	ProcessSoftDeleted(ctx context.Context, cutOff time.Time, fn func(userId sharedTypes.UUID) bool) error
	TrackClearSessions(ctx context.Context, userId sharedTypes.UUID, ip string, info interface{}) error
	BumpEpoch(ctx context.Context, userId sharedTypes.UUID) error
	CheckEmailAlreadyRegistered(ctx context.Context, email sharedTypes.Email) error
	GetUser(ctx context.Context, userId sharedTypes.UUID, target interface{}) error
	GetUserByEmail(ctx context.Context, email sharedTypes.Email, target interface{}) error
	GetContacts(ctx context.Context, userId sharedTypes.UUID) ([]WithPublicInfoAndNonStandardId, error)
	SetBetaProgram(ctx context.Context, userId sharedTypes.UUID, joined bool) error
	UpdateEditorConfig(ctx context.Context, userId sharedTypes.UUID, config EditorConfig) error
	TrackLogin(ctx context.Context, userId sharedTypes.UUID, epoch int64, ip string) error
	ChangeEmailAddress(ctx context.Context, change ForEmailChange, ip string, newEmail sharedTypes.Email) error
	SetUserName(ctx context.Context, userId sharedTypes.UUID, u WithNames) error
	ChangePassword(ctx context.Context, change ForPasswordChange, ip, operation string, newHashedPassword string) error
	DeleteDictionary(ctx context.Context, userId sharedTypes.UUID) error
	LearnWord(ctx context.Context, userId sharedTypes.UUID, word string) error
	UnlearnWord(ctx context.Context, userId sharedTypes.UUID, word string) error
	GetByPasswordResetToken(ctx context.Context, token oneTimeToken.OneTimeToken, u *ForPasswordChange) error
}

func New(db *pgxpool.Pool) Manager {
	return &manager{db: db}
}

var (
	ErrEmailAlreadyRegistered = &errors.InvalidStateError{
		Msg: "email already registered",
	}
)

func getErr(_ pgconn.CommandTag, err error) error {
	return err
}

func rewritePostgresErr(err error) error {
	if err == sql.ErrNoRows {
		return &errors.NotFoundError{}
	}
	return err
}

type manager struct {
	db *pgxpool.Pool
}

func (m *manager) CreateUser(ctx context.Context, u *ForCreation) error {
	_, err := m.db.Exec(ctx, `
WITH u AS (
    INSERT INTO users
        (beta_program, editor_config, email, email_created_at, epoch, features,
         first_name, id, last_login_at, last_login_ip, last_name,
         learned_words, login_count, must_reconfirm, password_hash,
         signup_date)
        VALUES (FALSE,
                $3,
                $1, $2, 1, $4, '', $5, $6, $7, '', ARRAY []::TEXT[],
                $8,
                FALSE, $9, $2)
        RETURNING id),
     log AS (
         INSERT INTO user_audit_log
             (id, info, initiator_id, ip_address, operation, timestamp,
              user_id)
             VALUES (gen_random_uuid(), '{}', $10, $11, $12, $2, $5)
             RETURNING FALSE)

INSERT
INTO one_time_tokens
(created_at, email, expires_at, token, use, user_id)
SELECT $2, $1, $13, $14, $15, u.id
FROM u;
`,
		string(u.Email),
		u.SignUpDate,
		&u.EditorConfig,
		&u.Features,
		u.Id,
		u.LastLoggedIn,
		u.LastLoginIp,
		u.LoginCount,
		u.HashedPassword,
		u.AuditLog[0].InitiatorId,
		u.AuditLog[0].IpAddress,
		u.AuditLog[0].Operation,
		u.SignUpDate.Add(7*24*time.Hour),
		u.OneTimeToken,
		u.OneTimeTokenUse,
	)
	if err != nil {
		if e, ok := err.(*pq.Error); ok {
			switch e.Constraint {
			case "user_email_key":
				return ErrEmailAlreadyRegistered
			case "one_time_tokens_pkey":
				return oneTimeToken.ErrDuplicateOneTimeToken
			}
		}
		return err
	}
	return nil
}

func (m *manager) ChangePassword(ctx context.Context, u ForPasswordChange, ip, operation string, newHashedPassword string) error {
	ok := false
	err := m.db.QueryRow(ctx, `
WITH u AS (
    UPDATE users
        SET epoch = epoch + 1, password_hash = $3
        WHERE id = $1 AND deleted_at IS NULL AND epoch = $2
        RETURNING id),
     log AS (
         INSERT INTO user_audit_log
             (id, info, initiator_id, ip_address, operation, timestamp,
              user_id)
             SELECT gen_random_uuid(),
                    '{}',
                    u.id,
                    $4,
                    $5,
                    transaction_timestamp(),
                    u.id
             FROM u),
     ott AS (
         UPDATE one_time_tokens
             SET used_at = transaction_timestamp()
             FROM u
             WHERE user_id = u.id
                 AND use = $6
                 AND used_at IS NULL)
SELECT TRUE
FROM u
`, u.Id, u.Epoch, newHashedPassword, ip, operation,
		oneTimeToken.PasswordResetUse).Scan(&ok)
	if err != nil {
		if err == sql.ErrNoRows {
			return ErrEpochChanged
		}
		return err
	}
	return nil
}

func (m *manager) UpdateEditorConfig(ctx context.Context, userId sharedTypes.UUID, e EditorConfig) error {
	blob, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return getErr(m.db.Exec(ctx, `
UPDATE users
SET editor_config = $2
WHERE id = $1
  AND deleted_at IS NULL
`, userId, string(blob)))
}

var ErrEpochChanged = &errors.InvalidStateError{Msg: "user epoch changed"}

func (m *manager) SoftDelete(ctx context.Context, userId sharedTypes.UUID, ip string) error {
	r, err := m.db.Exec(ctx, `
WITH u AS (
    UPDATE users
        SET deleted_at = transaction_timestamp(),
            epoch = epoch + 1
        WHERE id = $1
            AND deleted_at IS NULL
            AND (SELECT count(*) = 0
                 FROM projects p
                 WHERE p.owner_id = $1
                   AND p.deleted_at IS NULL)
        RETURNING id)

INSERT
INTO user_audit_log
(id, initiator_id, ip_address, operation, timestamp, user_id)
SELECT gen_random_uuid(),
       u.id,
       $2,
       $3,
       transaction_timestamp(),
       u.id
FROM u;
`, userId, ip, AuditLogOperationSoftDeletion)
	if err != nil {
		return err
	}
	if r.RowsAffected() == 0 {
		return &errors.UnprocessableEntityError{
			Msg: "user already deleted or user has owned projects",
		}
	}
	return nil
}

func (m *manager) HardDelete(ctx context.Context, userId sharedTypes.UUID) error {
	r, err := m.db.Exec(ctx, `
DELETE
FROM users
WHERE id = $1
  AND deleted_at IS NOT NULL
`, userId)
	if err != nil {
		return err
	}
	if r.RowsAffected() == 0 {
		return &errors.UnprocessableEntityError{
			Msg: "user missing or not deleted",
		}
	}
	return nil
}

func (m *manager) ProcessSoftDeleted(ctx context.Context, cutOff time.Time, fn func(userId sharedTypes.UUID) bool) error {
	ids := make([]sharedTypes.UUID, 0, 100)
	for {
		ids = ids[:0]
		r := m.db.QueryRow(ctx, `
WITH ids AS (SELECT id
             FROM users
             WHERE deleted_at <= $1
             ORDER BY deleted_at
             LIMIT 100)
SELECT array_agg(ids)
FROM ids
`, cutOff)
		if err := r.Scan(pq.Array(&ids)); err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		ok := true
		for _, userId := range ids {
			if !fn(userId) {
				ok = false
			}
		}
		if !ok {
			return nil
		}
	}
}

func (m *manager) TrackClearSessions(ctx context.Context, userId sharedTypes.UUID, ip string, info interface{}) error {
	blob, err := json.Marshal(info)
	if err != nil {
		return errors.Tag(err, "cannot serialize audit log info")
	}
	return getErr(m.db.Exec(ctx, `
WITH u AS (
    UPDATE users
        SET epoch = epoch + 1
        WHERE id = $1 AND deleted_at IS NULL
        RETURNING id)

INSERT
INTO user_audit_log
(id, info, initiator_id, ip_address, operation, timestamp, user_id)
SELECT gen_random_uuid(),
       $2,
       u.id,
       $3,
       $4,
       transaction_timestamp(),
       u.id
FROM u;
`, userId, blob, ip, AuditLogOperationClearSessions))
}

func (m *manager) ChangeEmailAddress(ctx context.Context, u ForEmailChange, ip string, newEmail sharedTypes.Email) error {
	blob, err := json.Marshal(changeEmailAddressAuditLogInfo{
		OldPrimaryEmail: u.Email,
		NewPrimaryEmail: newEmail,
	})
	if err != nil {
		return errors.Tag(err, "cannot serialize audit log info")
	}
	err = getErr(m.db.Exec(ctx, `
WITH u AS (
    UPDATE users
        SET
            email = $3,
            email_created_at = transaction_timestamp(),
            email_confirmed_at = NULL
        WHERE id = $1 AND deleted_at IS NULL AND epoch = $2
        RETURNING id),
     tokens AS (
         UPDATE one_time_tokens
             SET used_at = transaction_timestamp()
             WHERE user_id = u.id AND used_at IS NULL)
INSERT
INTO user_audit_log
(id, info, initiator_id, ip_address, operation, timestamp, user_id)
SELECT gen_random_uuid(),
       $4,
       u.id,
       $5,
       $6,
       transaction_timestamp(),
       u.id
FROM u
`, u.Id, u.Epoch, newEmail, blob, ip,
		AuditLogOperationChangePrimaryEmail))
	if err != nil {
		if e, ok := err.(*pq.Error); ok && e.Constraint == "user_email_key" {
			return ErrEmailAlreadyRegistered
		}
		if err == sql.ErrNoRows {
			return ErrEpochChanged
		}
		return err
	}
	return nil
}

const (
	AnonymousUserEpoch = 1
)

func (m *manager) BumpEpoch(ctx context.Context, userId sharedTypes.UUID) error {
	return getErr(m.db.Exec(ctx, `
UPDATE users
SET epoch = epoch + 1
WHERE id = $1
  AND deleted_at IS NULL;
`, userId))
}

func (m *manager) SetBetaProgram(ctx context.Context, userId sharedTypes.UUID, joined bool) error {
	return getErr(m.db.Exec(ctx, `
UPDATE users
SET beta_program = $2
WHERE id = $1
  AND deleted_at IS NULL;
`, userId, joined))
}

func (m *manager) SetUserName(ctx context.Context, userId sharedTypes.UUID, u WithNames) error {
	return getErr(m.db.Exec(ctx, `
UPDATE users
SET first_name = $2,
	last_name  = $3
WHERE id = $1
  AND deleted_at IS NULL;
`, userId, u.FirstName, u.LastName))
}

func (m *manager) TrackLogin(ctx context.Context, userId sharedTypes.UUID, epoch int64, ip string) error {
	return getErr(m.db.Exec(ctx, `
WITH u AS (
    UPDATE users
        SET login_count = login_count + 1,
            last_login_at = transaction_timestamp(),
            last_login_ip = $3
        WHERE id = $1 AND epoch = $2 AND deleted_at IS NULL
        RETURNING id)

INSERT
INTO user_audit_log
(id, initiator_id, ip_address, operation, timestamp, user_id)
SELECT gen_random_uuid(), u.id, $3, 'login', transaction_timestamp(), u.id
FROM u;
`, userId, epoch, ip))
}

func (m *manager) GetUser(ctx context.Context, userId sharedTypes.UUID, target interface{}) error {
	switch u := target.(type) {
	case *BetaProgramField:
		return rewritePostgresErr(m.db.QueryRow(ctx, `
SELECT beta_program
FROM users
WHERE id = $1
  AND deleted_at IS NULL
`, userId).Scan(&u.BetaProgram))
	case *HashedPasswordField:
		return rewritePostgresErr(m.db.QueryRow(ctx, `
SELECT password_hash
FROM users
WHERE id = $1
  AND deleted_at IS NULL
`, userId).Scan(&u.HashedPassword))
	case *LearnedWordsField:
		return rewritePostgresErr(m.db.QueryRow(ctx, `
SELECT learned_words
FROM users
WHERE id = $1
  AND deleted_at IS NULL
`, userId).Scan(pq.Array(&u.LearnedWords)))
	case *WithPublicInfo:
		return rewritePostgresErr(m.db.QueryRow(ctx, `
SELECT id, email, first_name, last_name
FROM users
WHERE id = $1
  AND deleted_at IS NULL
`, userId).Scan(&u.Id, &u.Email, &u.FirstName, &u.LastName))
	case *ForActivateUserPage:
		return rewritePostgresErr(m.db.QueryRow(ctx, `
SELECT email, login_count
FROM users
WHERE id = $1
  AND deleted_at IS NULL
`, userId).Scan(&u.Email, &u.LoginCount))
	case *ForEmailChange:
		return rewritePostgresErr(m.db.QueryRow(ctx, `
SELECT email, epoch, id
FROM users
WHERE id = $1
  AND deleted_at IS NULL
`, userId).Scan(&u.Email, &u.Epoch, &u.Id))
	case *ForPasswordChange:
		return rewritePostgresErr(m.db.QueryRow(ctx, `
SELECT id, email, first_name, last_name, epoch, password_hash
FROM users
WHERE id = $1
  AND deleted_at IS NULL
`, userId).Scan(
			&u.Id, &u.Email, &u.FirstName, &u.LastName,
			&u.Epoch, &u.HashedPassword,
		))
	case *ForSettingsPage:
		return rewritePostgresErr(m.db.QueryRow(ctx, `
SELECT id, email, first_name, last_name, beta_program
FROM users
WHERE id = $1
  AND deleted_at IS NULL
`, userId).Scan(
			&u.Id, &u.Email, &u.FirstName, &u.LastName, &u.BetaProgram,
		))
	default:
		return errors.New("missing query for target")
	}
}

func (m *manager) CheckEmailAlreadyRegistered(ctx context.Context, email sharedTypes.Email) error {
	x := false
	err := m.db.QueryRow(ctx, `
SELECT TRUE
FROM users
WHERE email = $1
`, email).Scan(&x)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	return ErrEmailAlreadyRegistered
}

func (m *manager) GetUserByEmail(ctx context.Context, email sharedTypes.Email, target interface{}) error {
	switch u := target.(type) {
	case *WithLoginInfo:
		return rewritePostgresErr(m.db.QueryRow(ctx, `
SELECT id, email, first_name, last_name, epoch, must_reconfirm, password_hash
FROM users
WHERE email = $1
  AND deleted_at IS NULL
`, email).Scan(
			&u.Id, &u.Email, &u.FirstName, &u.LastName,
			&u.Epoch, &u.MustReconfirm, &u.HashedPassword,
		))
	case *WithPublicInfo:
		return rewritePostgresErr(m.db.QueryRow(ctx, `
SELECT id, email, first_name, last_name
FROM users
WHERE email = $1
  AND deleted_at IS NULL
`, email).Scan(&u.Id, &u.Email, &u.FirstName, &u.LastName))
	default:
		return errors.New("missing query for target")
	}
}

func (m *manager) GetContacts(ctx context.Context, userId sharedTypes.UUID) ([]WithPublicInfoAndNonStandardId, error) {
	r, err := m.db.Query(ctx, `
WITH ids AS (SELECT unnest(ARRAY [a, b]) AS id
             FROM contacts
             WHERE a = $1
                OR b = $1
             ORDER BY connections DESC, last_touched DESC
             LIMIT 50)
SELECT u.id, email, first_name, last_name
FROM users u, ids
WHERE u.id = ids.id
  AND u.id != $1
  AND deleted_at IS NULL
`, userId)
	if err != nil {
		return nil, err
	}

	c := make([]WithPublicInfoAndNonStandardId, 0)
	defer r.Close()
	for i := 0; r.Next(); i++ {
		c = append(c, WithPublicInfoAndNonStandardId{})
		err = r.Scan(&c[i].Id, &c[i].Email, &c[i].FirstName, &c[i].LastName)
		if err != nil {
			return nil, err
		}
	}
	if err = r.Err(); err != nil {
		return nil, err
	}
	for i := 0; i < len(c); i++ {
		c[i].IdNoUnderscore = c[i].Id
	}
	return c, nil
}

func (m *manager) DeleteDictionary(ctx context.Context, userId sharedTypes.UUID) error {
	return getErr(m.db.Exec(ctx, `
UPDATE users
SET learned_words = ARRAY []::TEXT[]
WHERE id = $1
`, userId))
}

func (m *manager) LearnWord(ctx context.Context, userId sharedTypes.UUID, word string) error {
	return getErr(m.db.Exec(ctx, `
UPDATE users
SET learned_words = array_append(learned_words, $2)
WHERE id = $1
  AND array_position(learned_words, $2) IS NULL
`, userId, word))
}

func (m *manager) UnlearnWord(ctx context.Context, userId sharedTypes.UUID, word string) error {
	return getErr(m.db.Exec(ctx, `
UPDATE users
SET learned_words = array_remove(learned_words, $2)
WHERE id = $1
`, userId, word))
}

func (m *manager) GetByPasswordResetToken(ctx context.Context, token oneTimeToken.OneTimeToken, u *ForPasswordChange) error {
	return rewritePostgresErr(m.db.QueryRow(ctx, `
SELECT id, u.email, first_name, last_name, epoch, password_hash
FROM one_time_tokens ott
         INNER JOIN users u ON ott.user_id = u.id
WHERE ott.token = $1
  AND ott.use = $2
  AND ott.used_at IS NULL
  AND ott.expires_at > transaction_timestamp()
  AND u.deleted_at IS NULL
`, token, oneTimeToken.PasswordResetUse).Scan(
		&u.Id, &u.Email, &u.FirstName, &u.LastName,
		&u.Epoch, &u.HashedPassword,
	))
}
