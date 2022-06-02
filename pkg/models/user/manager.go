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

	"github.com/edgedb/edgedb-go"
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
	AddContact(ctx context.Context, userId, contactId sharedTypes.UUID) error
	ListProjects(ctx context.Context, userId sharedTypes.UUID, u interface{}) error
	SetBetaProgram(ctx context.Context, userId sharedTypes.UUID, joined bool) error
	UpdateEditorConfig(ctx context.Context, userId sharedTypes.UUID, config EditorConfig) error
	TrackLogin(ctx context.Context, userId sharedTypes.UUID, ip string) error
	ChangeEmailAddress(ctx context.Context, change *ForEmailChange, ip string, newEmail sharedTypes.Email) error
	SetUserName(ctx context.Context, userId sharedTypes.UUID, u *WithNames) error
	ChangePassword(ctx context.Context, change *ForPasswordChange, ip, operation string, newHashedPassword string) error
	DeleteDictionary(ctx context.Context, userId sharedTypes.UUID) error
	LearnWord(ctx context.Context, userId sharedTypes.UUID, word string) error
	UnlearnWord(ctx context.Context, userId sharedTypes.UUID, word string) error
	ResolveAndExpirePasswordResetToken(ctx context.Context, token oneTimeToken.OneTimeToken, u *ForPasswordChange) error
}

func New(db *sql.DB) Manager {
	return &manager{db: db}
}

var (
	ErrEmailAlreadyRegistered = &errors.InvalidStateError{
		Msg: "email already registered",
	}
)

func getErr(_ sql.Result, err error) error {
	return err
}

func rewritePostgresErr(err error) error {
	if err == sql.ErrNoRows {
		return &errors.NotFoundError{}
	}
	return err
}

func rewriteEdgedbError(err error) error {
	if err == nil {
		return nil
	}
	if e, ok := err.(edgedb.Error); ok && e.Category(edgedb.NoDataError) {
		return &errors.NotFoundError{}
	}
	return err
}

type manager struct {
	c  *edgedb.Client
	db *sql.DB
}

func (m *manager) CreateUser(ctx context.Context, u *ForCreation) error {
	// TODO: POST /api/register: cannot create user: pq: cannot insert multiple commands into a prepared statement
	_, err := m.db.ExecContext(ctx, `
BEGIN;

INSERT INTO users
(beta_program, editor_config, email, email_created_at, epoch, features,
 first_name, id, last_login_at, last_login_ip, last_name, learned_words,
 login_count, must_reconfirm, password_hash, signup_date)
VALUES (FALSE,
        ROW (TRUE, TRUE, FALSE, 12, 'lucida', 'normal', 'default', '', 'pdfjs', 'en', 'textmate'),
        $1, $2, 1, ROW ($3, $4), '', $5, $6, $7, '', ARRAY []::TEXT[], $8,
        FALSE, $9, $2);

INSERT INTO user_audit_log
(id, info, initiator_id, ip_address, operation, timestamp, user_id)
VALUES (gen_random_uuid(), '{}', $10, $11, $12, $2, $5);

INSERT INTO one_time_tokens
(created_at, email, expires_at, token, use, user_id)
VALUES ($2, $1, $13, $14, $15, $5);

END;
`,
		string(u.Email),
		u.SignUpDate,
		u.Features.CompileGroup,
		u.Features.CompileTimeout,
		u.Id.String(),
		u.LastLoggedIn,
		u.LastLoginIp,
		u.LoginCount,
		u.HashedPassword,
		u.AuditLog[0].InitiatorId.String(),
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

func (m *manager) ChangePassword(ctx context.Context, u *ForPasswordChange, ip, operation string, newHashedPassword string) error {
	r := make([]sharedTypes.UUID, 0)
	err := m.c.Query(ctx, `
with u := (select User filter .id = <uuid>$0 and .epoch = <int64>$1)
select (
	select (
		update u
		set {
			epoch := User.epoch + 1,
			password_hash := <str>$2,
		}
	).id
) union (
	select (
		insert UserAuditLogEntry {
			user := u,
			initiator := u,
			ip_address := <str>$3,
			operation := <str>$4,
		}
	).id
) union (
	for entry in ({1} if <str>$4 = 'reset-password' else <int64>{}) union (
		select (
			update u.email
			set {
				confirmed_at := datetime_of_transaction(),
			}
		).id
	)
)
`, &r, u.Id, u.Epoch, newHashedPassword, ip, operation)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	if len(r) == 0 {
		return &errors.NotFoundError{}
	}
	return nil
}

func (m *manager) UpdateEditorConfig(ctx context.Context, userId sharedTypes.UUID, e EditorConfig) error {
	return getErr(m.db.ExecContext(ctx, `
UPDATE users
SET editor_config = ROW (
    $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
    )
WHERE id = $1
  AND deleted_at IS NULL
`, userId.String(),
		e.AutoComplete, e.AutoPairDelimiters, e.SyntaxValidation,
		e.FontSize,
		e.FontFamily, e.LineHeight, e.Mode, e.OverallTheme, e.PDFViewer,
		e.SpellCheckLanguage, e.Theme,
	))
}

var ErrEpochChanged = &errors.InvalidStateError{Msg: "user epoch changed"}

func (m *manager) SoftDelete(ctx context.Context, userId sharedTypes.UUID, ip string) error {
	return getErr(m.db.ExecContext(ctx, `
BEGIN;

UPDATE users
SET deleted_at = transaction_timestamp(),
    epoch      = epoch + 1
WHERE id = $1
  AND deleted_at IS NULL;

INSERT INTO user_audit_log
(id, initiator_id, ip_address, operation, timestamp, user_id)
VALUES (gen_random_uuid(), $1, $2, $3, transaction_timestamp(),
        $1);

END;
`, userId.String(), ip, AuditLogOperationSoftDeletion))
}

func (m *manager) HardDelete(ctx context.Context, userId sharedTypes.UUID) error {
	r, err := m.db.ExecContext(ctx, `
DELETE
FROM users
WHERE id = $1
  AND deleted_at IS NOT NULL
`, userId.String())
	if err != nil {
		return err
	}
	if n, _ := r.RowsAffected(); n == 0 {
		return &errors.UnprocessableEntityError{
			Msg: "user missing or not deleted",
		}
	}
	return nil
}

func (m *manager) ProcessSoftDeleted(ctx context.Context, cutOff time.Time, fn func(userId sharedTypes.UUID) bool) error {
	userId := sharedTypes.UUID{}
	for {
		r, err := m.db.QueryContext(ctx, `
SELECT id
FROM users
WHERE deleted_at <= $1
ORDER BY deleted_at
LIMIT 100
`, cutOff)
		if err != nil {
			return rewriteEdgedbError(err)
		}
		ok := true
		for r.Next() {
			if err = r.Scan(&userId); err != nil {
				return err
			}
			if !fn(userId) {
				ok = false
			}
		}
		if err = r.Err(); err != nil {
			return err
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
	return getErr(m.db.ExecContext(ctx, `
BEGIN;

UPDATE users
SET epoch = epoch + 1
WHERE id = $1
  AND deleted_at IS NULL;

INSERT INTO user_audit_log
(id, info, initiator_id, ip_address, operation, timestamp, user_id)
VALUES (gen_random_uuid(), $2, $1, $3, $4,
        transaction_timestamp(), $1);

END;
`, userId.String(), blob, ip, AuditLogOperationClearSessions))
}

func (m *manager) ChangeEmailAddress(ctx context.Context, u *ForEmailChange, ip string, newEmail sharedTypes.Email) error {
	err := m.c.Query(ctx, `
with
	u := assert_exists((
		select User
		filter .id = <uuid>$0 and .epoch = <int64>$1
	)),
	oldPrimaryEmail := u.email,
	newPrimaryEmail := (insert Email {
		email := <str>$2,
	}),
	uChanged := (
		update u
		set {
			email := newPrimaryEmail,
			emails := { newPrimaryEmail },
		}
	),
	oldPrimaryEmailDeleted := (
		delete oldPrimaryEmail
		filter exists uChanged
	),
	auditLog := (
		insert UserAuditLogEntry {
			user := u,
			initiator := u,
			ip_address := <str>$3,
			operation := <str>$4,
			info := <json>{
				oldPrimaryEmail := oldPrimaryEmail.email,
				newPrimaryEmail := newPrimaryEmail.email,
			}
		}
	)
select {exists oldPrimaryEmailDeleted, exists auditLog}
`,
		&[]bool{},
		u.Id, u.Epoch, newEmail, ip, AuditLogOperationChangePrimaryEmail,
	)
	if err != nil {
		if e, ok := err.(edgedb.Error); ok && e.Category(edgedb.ConstraintViolationError) {
			return ErrEmailAlreadyRegistered
		}
		err = rewriteEdgedbError(err)
		if errors.IsNotFoundError(err) {
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
	return getErr(m.db.ExecContext(ctx, `
UPDATE users
SET epoch = epoch + 1
WHERE id = $1
  AND deleted_at IS NULL;
`, userId.String()))
}

func (m *manager) SetBetaProgram(ctx context.Context, userId sharedTypes.UUID, joined bool) error {
	return getErr(m.db.ExecContext(ctx, `
UPDATE users
SET beta_program = $2
WHERE id = $1
  AND deleted_at IS NULL;
`, userId.String(), joined))
}

func (m *manager) SetUserName(ctx context.Context, userId sharedTypes.UUID, u *WithNames) error {
	return getErr(m.db.ExecContext(ctx, `
UPDATE users
SET first_name = $2,
	last_name  = $3
WHERE id = $1
  AND deleted_at IS NULL;
`, userId.String(), u.FirstName, u.LastName))
}

func (m *manager) TrackLogin(ctx context.Context, userId sharedTypes.UUID, ip string) error {
	return getErr(m.db.ExecContext(ctx, `
BEGIN;

UPDATE users
SET login_count   = login_count + 1,
    last_login_at = transaction_timestamp(),
    last_login_ip = $2
WHERE id = $1
  AND deleted_at IS NULL;

INSERT INTO user_audit_log
(id, initiator_id, ip_address, operation, timestamp, user_id)
VALUES (gen_random_uuid(), $1, $2, 'login', transaction_timestamp(), $1);

END;
`, userId.String(), ip))
}

func (m *manager) GetUser(ctx context.Context, userId sharedTypes.UUID, target interface{}) error {
	switch u := target.(type) {
	case *BetaProgramField:
		return rewritePostgresErr(m.db.QueryRowContext(ctx, `
SELECT beta_program
FROM users
WHERE id = $1
  AND deleted_at IS NULL
`, userId.String()).Scan(&u.BetaProgram))
	case *HashedPasswordField:
		return rewritePostgresErr(m.db.QueryRowContext(ctx, `
SELECT password_hash
FROM users
WHERE id = $1
  AND deleted_at IS NULL
`, userId.String()).Scan(&u.HashedPassword))
	case *LearnedWordsField:
		return rewritePostgresErr(m.db.QueryRowContext(ctx, `
SELECT learned_words
FROM users
WHERE id = $1
  AND deleted_at IS NULL
`, userId.String()).Scan(pq.Array(&u.LearnedWords)))
	case *WithPublicInfo:
		return rewritePostgresErr(m.db.QueryRowContext(ctx, `
SELECT id, email, first_name, last_name
FROM users
WHERE id = $1
  AND deleted_at IS NULL
`, userId.String()).Scan(&u.Id, &u.Email, &u.FirstName, &u.LastName))
	case *ForActivateUserPage:
		return rewritePostgresErr(m.db.QueryRowContext(ctx, `
SELECT email, login_count
FROM users
WHERE id = $1
  AND deleted_at IS NULL
`, userId.String()).Scan(&u.Email, &u.LoginCount))
	case *ForEmailChange:
		return rewritePostgresErr(m.db.QueryRowContext(ctx, `
SELECT email, epoch, id
FROM users
WHERE id = $1
  AND deleted_at IS NULL
`, userId.String()).Scan(&u.Email, &u.Epoch, &u.Id))
	case *ForPasswordChange:
		return rewritePostgresErr(m.db.QueryRowContext(ctx, `
SELECT id, email, first_name, last_name, epoch, password_hash
FROM users
WHERE id = $1
  AND deleted_at IS NULL
`, userId.String()).Scan(
			&u.Id, &u.Email, &u.FirstName, &u.LastName,
			&u.Epoch, &u.HashedPassword,
		))
	case *ForSettingsPage:
		return rewritePostgresErr(m.db.QueryRowContext(ctx, `
SELECT id, email, first_name, last_name, beta_program
FROM users
WHERE id = $1
  AND deleted_at IS NULL
`, userId.String()).Scan(
			&u.Id, &u.Email, &u.FirstName, &u.LastName, &u.BetaProgram,
		))
	default:
		return errors.New("missing query for target")
	}
}

func (m *manager) CheckEmailAlreadyRegistered(ctx context.Context, email sharedTypes.Email) error {
	x := false
	err := m.db.QueryRowContext(ctx, `
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
		return rewritePostgresErr(m.db.QueryRowContext(ctx, `
SELECT id, email, first_name, last_name, epoch, must_reconfirm, password_hash
FROM users
WHERE email = $1
  AND deleted_at IS NULL
`, email).Scan(
			&u.Id, &u.Email, &u.FirstName, &u.LastName,
			&u.Epoch, &u.MustReconfirm, &u.HashedPassword,
		))
	case *WithPublicInfo:
		return rewritePostgresErr(m.db.QueryRowContext(ctx, `
SELECT id, email, first_name, last_name
FROM users
WHERE email = $1
  AND deleted_at IS NULL
`, email).Scan(&u.Id, &u.Email, &u.FirstName, &u.LastName))
	default:
		return errors.New("missing query for target")
	}
}

func (m *manager) ListProjects(ctx context.Context, userId sharedTypes.UUID, u interface{}) error {
	err := m.c.QuerySingle(ctx, `
with u := (select User filter .id = <uuid>$0 and not exists .deleted_at)
select u {
	email: { email },
	emails: { email, confirmed_at },
	first_name,
	id,
	last_name,
	projects: {
		access_ro: { id } filter User = u,
		access_rw: { id } filter User = u,
		access_token_ro: { id } filter User = u,
		access_token_rw: { id } filter User = u,
		archived_by: { id } filter User = u,
		epoch,
		id,
		last_updated_at,
		last_updated_by: {
			email: { email },
			first_name,
			id,
			last_name,
		},
		name,
		owner: {
			email: { email },
			first_name,
			id,
			last_name,
		},
		public_access_level,
		trashed_by: { id } filter User = u,
	},
	tags: {
		id,
		name,
		projects,
	},
}
`, u, userId)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	return nil
}

func (m *manager) GetContacts(ctx context.Context, userId sharedTypes.UUID) ([]WithPublicInfoAndNonStandardId, error) {
	r, err := m.db.QueryContext(ctx, `
WITH ids AS (SELECT unnest(ARRAY [a, b])
             FROM contacts
             WHERE a = $1
                OR b = $1
             ORDER BY connections DESC, last_touched DESC
             LIMIT 50)
SELECT id, email, first_name, last_name
FROM users
WHERE id = ids
  AND id != $1
  AND deleted_at IS NULL
`, userId.String())
	if err != nil {
		return nil, err
	}

	c := make([]WithPublicInfoAndNonStandardId, 0)
	defer func() { _ = r.Close() }()
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

func (m *manager) AddContact(ctx context.Context, userId, contactId sharedTypes.UUID) error {
	a, b := userId, contactId
	for i, x := range userId {
		if contactId[i] > x {
			a, b = contactId, userId
			break
		}
	}
	return getErr(m.db.ExecContext(ctx, `
INSERT INTO contacts
VALUES ($1, $2, 1, now())
ON CONFLICT DO UPDATE
    SET connections  = connections + 1,
        last_touched = now()
`, a.String(), b.String()))
}

func (m *manager) DeleteDictionary(ctx context.Context, userId sharedTypes.UUID) error {
	return getErr(m.db.ExecContext(ctx, `
UPDATE users
SET learned_words = NULL
WHERE id = $1
`, userId.String()))
}

func (m *manager) LearnWord(ctx context.Context, userId sharedTypes.UUID, word string) error {
	return getErr(m.db.ExecContext(ctx, `
UPDATE users
SET learned_words = array_append(learned_words, $2)
WHERE id = $1
  AND array_position(learned_words, $2) IS NULL
`, userId.String(), word))
}

func (m *manager) UnlearnWord(ctx context.Context, userId sharedTypes.UUID, word string) error {
	return getErr(m.db.ExecContext(ctx, `
UPDATE users
SET learned_words = array_remove(learned_words, $2)
WHERE id = $1
`, userId.String(), word))
}

func (m *manager) ResolveAndExpirePasswordResetToken(ctx context.Context, token oneTimeToken.OneTimeToken, u *ForPasswordChange) error {
	err := m.c.QuerySingle(ctx, `
with
	t := (
		update OneTimeToken
		filter
				.use = <str>$0
			and .token = <str>$1
			and not exists .used_at
			and .expires_at > datetime_of_transaction()
			and not exists .email.user.deleted_at
		set {
			used_at := datetime_of_transaction()
		}
	)
select t.email.user {
	email: { email },
	epoch,
	first_name,
	id,
	last_name,
	password_hash,
}
`, u, oneTimeToken.PasswordResetUse, token)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	return nil
}
