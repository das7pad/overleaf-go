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
	ids := make([]IdField, 3)
	err := m.c.Query(ctx, `
with
	e := (insert Email {
		email := <str>$6,
	}),
	u := (insert User {
		editor_config := (insert EditorConfig {
			auto_complete := true,
			auto_pair_delimiters := true,
			font_family := 'lucida',
			font_size := 12,
			line_height := 'normal',
			mode := 'default',
			overall_theme := '',
			pdf_viewer := 'pdfjs',
			syntax_validation := false,
			spell_check_language := 'en',
			theme := 'textmate',
		}),
		email := e,
		emails := { e },
		features := (insert Features {
			compile_group := <str>$4,
			compile_timeout := <duration>$5,
		}),
		first_name := <str>$0,
		last_name := <str>$1,
		password_hash := <str>$2,
		last_logged_in := (
			<datetime>{} if <str>$3 = "" else {datetime_of_transaction()}
		),
		last_login_ip := <str>$3,
		login_count := ({0} if <str>$3 = "" else {1}),
	}),
	auditLog := (
		for entry in ({1} if <str>$3 != "" else <int64>{}) union (
			insert UserAuditLogEntry {
				user := u,
				initiator := u,
				operation := <str>$7,
				ip_address := <str>$3,
			}
		)
	)
select {u,auditLog}
`,
		&ids,
		u.FirstName,
		u.LastName,
		u.HashedPassword,
		u.LastLoginIp,
		u.Features.CompileGroup,
		u.Features.CompileTimeout,
		string(u.Email),
		AuditLogOperationLogin,
	)
	if err != nil {
		if e, ok := err.(edgedb.Error); ok && e.Category(edgedb.ConstraintViolationError) {
			return ErrEmailAlreadyRegistered
		}
		return rewriteEdgedbError(err)
	}
	u.Id = ids[0].Id
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
	return rewriteEdgedbError(m.c.QuerySingle(ctx, `
with u := (select User filter .id = <uuid>$0 and not exists .deleted_at)
update u.editor_config
set {
	auto_complete := <bool>$1,
	auto_pair_delimiters := <bool>$2,
	font_family := <str>$3,
	font_size := <int64>$4,
	line_height := <str>$5,
	mode := <str>$6,
	overall_theme := <str>$7,
	pdf_viewer := <str>$8,
	syntax_validation := <bool>$9,
	spell_check_language := <str>$10,
	theme := <str>$11,
}
`,
		&IdField{},
		userId,
		e.AutoComplete, e.AutoPairDelimiters, e.FontFamily, e.FontSize,
		e.LineHeight, e.Mode, e.OverallTheme, e.PDFViewer, e.SyntaxValidation,
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
VALUES (gen_random_uuid(), $1, $2, 'soft-deletion', transaction_timestamp(),
        $1);

END;
`, userId.String(), ip))
}

func (m *manager) HardDelete(ctx context.Context, userId sharedTypes.UUID) error {
	r := m.db.QueryRowContext(ctx, `
DELETE
FROM users
WHERE id = $1
  AND deleted_at IS NOT NULL
RETURNING 1
`, userId.String())
	found, err := readInt(r)
	if err != nil {
		return err
	}
	if found == 0 {
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
VALUES (gen_random_uuid(), $2, $1, $3, 'clear-sessions',
        transaction_timestamp(), $1);

END;
`, userId.String(), blob, ip))
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
	var q string
	switch dst := target.(type) {
	case *BetaProgramField:
		q = `
select User { beta_program }
filter .id = <uuid>$0 and not exists .deleted_at`
	case *HashedPasswordField:
		q = `
select User { password_hash }
filter .id = <uuid>$0 and not exists .deleted_at`
	case *LearnedWordsField:
		r := m.db.QueryRowContext(ctx, `
SELECT learned_words
FROM users
WHERE id = $1
  AND deleted_at IS NULL
`, userId.String())
		if err := r.Err(); err != nil {
			return err
		}
		if err := r.Scan(pq.Array(&dst.LearnedWords)); err != nil {
			return err
		}
		return nil
	case *WithPublicInfoAndNonStandardId, *WithPublicInfo:
		q = `
select User { email: { email }, id, first_name, last_name }
filter .id = <uuid>$0 and not exists .deleted_at`
	case *ForActivateUserPage:
		q = `
select User { email: { email }, login_count }
filter .id = <uuid>$0 and not exists .deleted_at`
	case *ForEmailChange:
		q = `
select User { email: { email }, epoch, id }
filter .id = <uuid>$0 and not exists .deleted_at`
	case *ForPasswordChange:
		q = `
select User {
	email: { email }, epoch, id, first_name, last_name, password_hash
}
filter .id = <uuid>$0 and not exists .deleted_at`
	case *ForSettingsPage:
		q = `
select User { beta_program, email: { email }, id, first_name, last_name }
filter .id = <uuid>$0 and not exists .deleted_at`
	default:
		return errors.New("missing query for target")
	}
	if err := m.c.QuerySingle(ctx, q, target, userId); err != nil {
		return rewriteEdgedbError(err)
	}
	return nil
}

func readInt(r *sql.Row) (int, error) {
	if err := r.Err(); err != nil {
		return 0, err
	}
	found := 0
	if err := r.Scan(&found); err != nil {
		return 0, err
	}
	return found, nil
}

func (m *manager) CheckEmailAlreadyRegistered(ctx context.Context, email sharedTypes.Email) error {
	r := m.db.QueryRowContext(ctx, `
SELECT count(*)
FROM users
WHERE email = $1
LIMIT 1
`, email)
	found, err := readInt(r)
	if err != nil {
		return err
	}
	if found != 0 {
		return ErrEmailAlreadyRegistered
	}
	return nil
}

func (m *manager) GetUserByEmail(ctx context.Context, email sharedTypes.Email, target interface{}) error {
	var q string
	switch target.(type) {
	case *WithLoginInfo:
		q = `
select User {
	email: { email },
	epoch,
	first_name,
	id,
	last_name,
	must_reconfirm,
	password_hash,
}
filter .email.email = <str>$0 and not exists .deleted_at
limit 1`
	case *WithPublicInfoAndNonStandardId, *WithPublicInfo:
		q = `
select User { email: { email }, id, first_name, last_name }
filter .email.email = <str>$0 and not exists .deleted_at
limit 1`
	default:
		return errors.New("missing query for target")
	}
	if err := m.c.QuerySingle(ctx, q, target, email); err != nil {
		return rewriteEdgedbError(err)
	}
	return nil
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

func (m manager) DeleteDictionary(ctx context.Context, userId sharedTypes.UUID) error {
	return getErr(m.db.ExecContext(ctx, `
UPDATE users
SET learned_words = NULL
WHERE id = $1
`, userId.String()))
}

func (m manager) LearnWord(ctx context.Context, userId sharedTypes.UUID, word string) error {
	return getErr(m.db.ExecContext(ctx, `
UPDATE users
SET learned_words = array_append(learned_words, $2)
WHERE id = $1
  AND array_position(learned_words, $2) IS NULL
`, userId.String(), word))
}

func (m manager) UnlearnWord(ctx context.Context, userId sharedTypes.UUID, word string) error {
	return getErr(m.db.ExecContext(ctx, `
UPDATE users
SET learned_words = array_remove(learned_words, $2)
WHERE id = $1
`, userId.String(), word))
}
