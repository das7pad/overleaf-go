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
	"encoding/json"
	"sort"
	"time"

	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Manager interface {
	CreateUser(ctx context.Context, u *ForCreation) error
	SoftDelete(ctx context.Context, userId edgedb.UUID, ip string) error
	HardDelete(ctx context.Context, userId edgedb.UUID) error
	ProcessSoftDeleted(ctx context.Context, cutOff time.Time, fn func(userId edgedb.UUID) bool) error
	TrackClearSessions(ctx context.Context, userId edgedb.UUID, ip string, info interface{}) error
	BumpEpoch(ctx context.Context, userId edgedb.UUID) error
	CheckEmailAlreadyRegistered(ctx context.Context, email sharedTypes.Email) error
	GetUser(ctx context.Context, userId edgedb.UUID, target interface{}) error
	GetUserByEmail(ctx context.Context, email sharedTypes.Email, target interface{}) error
	GetContacts(ctx context.Context, userId edgedb.UUID) ([]Contact, error)
	AddContact(ctx context.Context, userId, contactId edgedb.UUID) error
	ListProjects(ctx context.Context, userId edgedb.UUID, u interface{}) error
	SetBetaProgram(ctx context.Context, userId edgedb.UUID, joined bool) error
	UpdateEditorConfig(ctx context.Context, userId edgedb.UUID, config EditorConfig) error
	TrackLogin(ctx context.Context, userId edgedb.UUID, ip string) error
	ChangeEmailAddress(ctx context.Context, change *ForEmailChange, ip string, newEmail sharedTypes.Email) error
	SetUserName(ctx context.Context, userId edgedb.UUID, u *WithNames) error
	ChangePassword(ctx context.Context, change *ForPasswordChange, ip, operation string, newHashedPassword string) error
}

func New(c *edgedb.Client) Manager {
	return &manager{
		c: c,
	}
}

var (
	ErrEmailAlreadyRegistered = &errors.InvalidStateError{
		Msg: "email already registered",
	}
)

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
	c *edgedb.Client
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
	r := make([]edgedb.UUID, 0)
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

func (m *manager) UpdateEditorConfig(ctx context.Context, userId edgedb.UUID, e EditorConfig) error {
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

func (m *manager) SoftDelete(ctx context.Context, userId edgedb.UUID, ip string) error {
	return rewriteEdgedbError(m.c.QuerySingle(ctx, `
with
	u := (
		update User
		filter .id = <uuid>$0 and not exists .deleted_at
		set {
			deleted_at := datetime_of_transaction(),
			epoch := User.epoch + 1,
		}
	)
insert UserAuditLogEntry {
	user := u,
	initiator := u,
	ip_address := <str>$1,
	operation := <str>$2,
}
`, &IdField{}, userId, ip, AuditLogOperationSoftDeletion))
}

type hardDeleteResult struct {
	UserExists            bool   `edgedb:"user_exists"`
	UserSoftDeleted       bool   `edgedb:"user_soft_deleted"`
	UserNotJoinedProjects bool   `edgedb:"user_not_joined_projects"`
	Deletions             []bool `edgedb:"deletions"`
}

func (m *manager) HardDelete(ctx context.Context, userId edgedb.UUID) error {
	r := hardDeleteResult{}
	err := m.c.QuerySingle(ctx, `
with
	u := (select User filter .id = <uuid>$0),
	uSoftDeleted := (select u filter exists .deleted_at),
	uNotJoinedProjects := (select uSoftDeleted filter not exists .projects),
	editorConfigCleanup := (delete uNotJoinedProjects.editor_config),
	featureCleanup := (delete uNotJoinedProjects.features),
	secondaryEmailCleanup := (
		delete uNotJoinedProjects.emails
		filter Email != u.email
	),
	primaryEmail := u.email,
	uDeleted := (delete uNotJoinedProjects),
	primaryEmailCleanup := (
		delete primaryEmail filter exists uDeleted
	),
select {
	user_exists := exists u,
	user_soft_deleted := exists uSoftDeleted,
	user_not_joined_projects := exists uNotJoinedProjects,
	deletions := {
		exists editorConfigCleanup,
		exists secondaryEmailCleanup,
		exists primaryEmailCleanup,
		exists featureCleanup,
		exists uDeleted,
	},
}
`, &r, userId)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	switch {
	case !r.UserExists:
		return &errors.UnprocessableEntityError{Msg: "user missing"}
	case !r.UserSoftDeleted:
		return &errors.UnprocessableEntityError{Msg: "user not soft deleted"}
	case !r.UserNotJoinedProjects:
		return &errors.UnprocessableEntityError{Msg: "user joined projects"}
	}
	return nil
}

func (m *manager) ProcessSoftDeleted(ctx context.Context, cutOff time.Time, fn func(userId edgedb.UUID) bool) error {
	ids := make([]edgedb.UUID, 0, 100)
	for {
		ids = ids[:0]
		err := m.c.Query(ctx, `
select (
	select User
	filter .deleted_at <= <datetime>$0
	order by .deleted_at
	limit 100
).id
`, &ids, cutOff)
		if err != nil {
			return rewriteEdgedbError(err)
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

func (m *manager) TrackClearSessions(ctx context.Context, userId edgedb.UUID, ip string, info interface{}) error {
	blob, err := json.Marshal(info)
	if err != nil {
		return errors.Tag(err, "cannot serialize audit log info")
	}
	return rewriteEdgedbError(m.c.QuerySingle(ctx, `
with
	u := (
		updater User
		filter .id = <uuid>$0 and not exists .deleted_at
		set {
			epoch := User.epoch + 1,
		}
	)
insert UserAuditLogEntry {
	user := u,
	initiator := u,
	operation := <str>$3,
	ip_address := <str>$1,
	info := <json>$2,
}
`, &IdField{}, userId, ip, blob, AuditLogOperationClearSessions))
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

func (m *manager) BumpEpoch(ctx context.Context, userId edgedb.UUID) error {
	return rewriteEdgedbError(m.c.QuerySingle(ctx, `
update User
filter .id = <uuid>$0 and not exists .deleted_at
set {
	epoch := User.epoch + 1,
}
`, &IdField{}, userId))
}

func (m *manager) SetBetaProgram(ctx context.Context, userId edgedb.UUID, joined bool) error {
	return rewriteEdgedbError(m.c.QuerySingle(ctx, `
update User
filter .id = <uuid>$0 and not exists .deleted_at
set {
	beta_program := <bool>$1,
}
`, &IdField{}, userId, joined))
}

func (m *manager) SetUserName(ctx context.Context, userId edgedb.UUID, u *WithNames) error {
	return rewriteEdgedbError(m.c.QuerySingle(ctx, `
update User
filter .id = <uuid>$0 and not exists .deleted_at
set {
	first_name := <str>$1,
	last_name := <str>$2,
}
`, &IdField{}, userId, u.FirstName, u.LastName))
}

func (m *manager) TrackLogin(ctx context.Context, userId edgedb.UUID, ip string) error {
	err := m.c.QuerySingle(ctx, `
with
	u := (
		update User
		filter .id = <uuid>$0 and not exists .deleted_at
		set {
			login_count := User.login_count + 1,
			last_logged_in := datetime_of_transaction(),
			last_login_ip := <str>$1,
		}
	)
insert UserAuditLogEntry {
	user := u,
	initiator := u,
	operation := 'login',
	ip_address := <str>$1,
}
`, &IdField{}, userId, ip)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	return nil
}

func (m *manager) GetUser(ctx context.Context, userId edgedb.UUID, target interface{}) error {
	var q string
	switch target.(type) {
	case *BetaProgramField:
		q = `
select User { beta_program }
filter .id = <uuid>$0 and not exists .deleted_at`
	case *HashedPasswordField:
		q = `
select User { password_hash }
filter .id = <uuid>$0 and not exists .deleted_at`
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

func (m *manager) CheckEmailAlreadyRegistered(ctx context.Context, email sharedTypes.Email) error {
	exists := false
	err := m.c.QuerySingle(ctx, `
select exists (select User filter .email.email = <str>$0)
`, &exists, email)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	if exists {
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

func (m *manager) ListProjects(ctx context.Context, userId edgedb.UUID, u interface{}) error {
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

func (m *manager) GetContacts(ctx context.Context, userId edgedb.UUID) ([]Contact, error) {
	u := ContactsField{}
	err := m.c.QuerySingle(ctx, `
select User {
	contacts: {
		@connections,
		@last_touched,
		email: { email },
		id,
		first_name,
		last_name,
	},
}
filter .id = <uuid>$0 and not exists .deleted_at
`, &u, userId)
	if err != nil {
		return nil, rewriteEdgedbError(err)
	}

	sort.Slice(u.Contacts, func(i, j int) bool {
		return u.Contacts[i].IsPreferredOver(u.Contacts[j])
	})

	if len(u.Contacts) > 50 {
		u.Contacts = u.Contacts[:50]
	}
	for i := 0; i < len(u.Contacts); i++ {
		u.Contacts[i].IdNoUnderscore = u.Contacts[i].Id
	}
	return u.Contacts, nil
}

func (m *manager) AddContact(ctx context.Context, userId, contactId edgedb.UUID) error {
	var r bool
	err := m.c.QuerySingle(ctx, `
with
	existing := (
		select User.contacts@connections
		filter User.id = <uuid>$0 and User.contacts.id = <uuid>$1
		limit 1
	),
select exists (
	select (
		update User
		filter .id = <uuid>$0 and not exists .deleted_at
		set {
			contacts += (
				select detached User {
					@connections := (existing ?? 0) + 1,
					@last_touched := datetime_of_transaction(),
				}
				filter .id = <uuid>$1 and not exists .deleted_at
			)
		}
	) union (
		update User
		filter .id = <uuid>$1 and not exists .deleted_at
		set {
			contacts += (
				select detached User {
					@connections := (existing ?? 0) + 1,
					@last_touched := datetime_of_transaction(),
				}
				filter .id = <uuid>$0 and not exists .deleted_at
			)
		}
	)
)
`, &r, userId, contactId)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	return nil
}
