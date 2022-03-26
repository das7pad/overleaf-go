// Golang port of Overleaf
// Copyright (C) 2021 Jakob Ackermann <das7pad@outlook.com>
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
	"sort"
	"time"

	"github.com/edgedb/edgedb-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Manager interface {
	CreateUser(ctx context.Context, u *ForCreation) error
	Delete(ctx context.Context, userId edgedb.UUID, epoch int64) error
	TrackClearSessions(ctx context.Context, userId edgedb.UUID, ip string, info interface{}) error
	BumpEpoch(ctx context.Context, userId edgedb.UUID) error
	GetEpoch(ctx context.Context, userId edgedb.UUID) (int64, error)
	GetUser(ctx context.Context, userId edgedb.UUID, target interface{}) error
	GetUserByEmail(ctx context.Context, email sharedTypes.Email, target interface{}) error
	GetUsersWithPublicInfo(ctx context.Context, users []edgedb.UUID) ([]WithPublicInfo, error)
	GetUsersForBackFilling(ctx context.Context, ids UniqUserIds) (UsersForBackFilling, error)
	GetUsersForBackFillingNonStandardId(ctx context.Context, ids UniqUserIds) (UsersForBackFillingNonStandardId, error)
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

func New(c *edgedb.Client, db *mongo.Database) Manager {
	return &manager{
		c:   c,
		col: db.Collection("users"),
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
	c   *edgedb.Client
	col *mongo.Collection
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
		for entry in (<int64>{} if <str>$3 = "" else {1}) union (
			update User
			filter User = u
			set {
				audit_log += (
					insert UserAuditLogEntry {
						initiator := u,
						operation := <str>$7,
						ip_address := <str>$4,
					}
				)
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
	now := time.Now().UTC()
	q := &withIdAndEpoch{
		IdField: IdField{
			Id: u.Id,
		},
		EpochField: EpochField{
			Epoch: u.Epoch,
		},
	}
	set := bson.M{
		"hashedPassword": newHashedPassword,
	}
	var filters bson.A
	if operation == AuditLogOperationResetPassword {
		set["emails.$[email].confirmedAt"] = now
		set["emails.$[email].reconfirmedAt"] = now
		filters = append(filters, bson.M{
			"email.email": u.Email,
		})
	}
	_, err := m.col.UpdateOne(ctx, q, bson.M{
		"$set": set,
		"$inc": EpochField{
			Epoch: 1,
		},
		"$push": bson.M{
			"auditLog": bson.M{
				"$each": bson.A{
					AuditLogEntry{
						InitiatorId: u.Id,
						IpAddress:   ip,
						Operation:   operation,
						Timestamp:   now,
					},
				},
				"$slice": -MaxAuditLogEntries,
			},
		},
	}, options.Update().SetArrayFilters(options.ArrayFilters{
		Filters: filters,
	}))
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}

func (m *manager) UpdateEditorConfig(ctx context.Context, userId edgedb.UUID, editorConfig EditorConfig) error {
	q := &IdField{Id: userId}
	u := bson.M{
		"$set": &EditorConfigField{
			EditorConfig: editorConfig,
		},
	}
	_, err := m.col.UpdateOne(ctx, q, u)
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}

var ErrEpochChanged = &errors.InvalidStateError{Msg: "user epoch changed"}

func (m *manager) Delete(ctx context.Context, userId edgedb.UUID, epoch int64) error {
	q := &withIdAndEpoch{
		IdField: IdField{
			Id: userId,
		},
		EpochField: EpochField{
			Epoch: epoch,
		},
	}
	r, err := m.col.DeleteOne(ctx, q)
	if err != nil {
		return rewriteMongoError(err)
	}
	if r.DeletedCount != 1 {
		return ErrEpochChanged
	}
	return nil
}

func (m *manager) TrackClearSessions(ctx context.Context, userId edgedb.UUID, ip string, info interface{}) error {
	now := time.Now().UTC()
	_, err := m.col.UpdateOne(ctx, &IdField{Id: userId}, &bson.M{
		"$inc": EpochField{Epoch: 1},
		"$push": bson.M{
			"auditLog": bson.M{
				"$each": bson.A{
					AuditLogEntry{
						Info:        info,
						InitiatorId: userId,
						IpAddress:   ip,
						Operation:   AuditLogOperationClearSessions,
						Timestamp:   now,
					},
				},
				"$slice": -MaxAuditLogEntries,
			},
		},
	})
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}

func (m *manager) ChangeEmailAddress(ctx context.Context, u *ForEmailChange, ip string, newEmail sharedTypes.Email) error {
	now := time.Now().UTC()
	q := &withIdAndEpoch{
		IdField: IdField{
			Id: u.Id,
		},
		EpochField: EpochField{
			Epoch: u.Epoch,
		},
	}
	r, err := m.col.UpdateOne(ctx, q, bson.M{
		"$set": withEmailFields{
			EmailField{
				Email: newEmail,
			},
			EmailsField{
				Emails: []EmailDetails{
					{
						// TODO: refactor into server side gen.
						Id:        edgedb.UUID{},
						CreatedAt: now,
						Email:     newEmail,
					},
				},
			},
		},
		"$inc": EpochField{Epoch: 1},
		"$push": bson.M{
			"auditLog": bson.M{
				"$each": bson.A{
					AuditLogEntry{
						Info: bson.M{
							"oldPrimaryEmail": u.Email,
							"newPrimaryEmail": newEmail,
						},
						InitiatorId: u.Id,
						IpAddress:   ip,
						Operation:   AuditLogOperationChangePrimaryEmail,
						Timestamp:   now,
					},
				},
				"$slice": -MaxAuditLogEntries,
			},
		},
	})
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return ErrEmailAlreadyRegistered
		}
		return rewriteMongoError(err)
	}
	if r.ModifiedCount != 1 {
		return ErrEpochChanged
	}
	return nil
}

const (
	AnonymousUserEpoch = 1
	MaxAuditLogEntries = 200
)

func (m *manager) BumpEpoch(ctx context.Context, userId edgedb.UUID) error {
	return rewriteEdgedbError(m.c.QuerySingle(ctx, `
update User
filter .id = <uuid>$0
set {
	epoch := User.epoch + 1,
}
`, &IdField{}, userId))
}

func (m *manager) SetBetaProgram(ctx context.Context, userId edgedb.UUID, joined bool) error {
	return rewriteEdgedbError(m.c.QuerySingle(ctx, `
update User
filter .id = <uuid>$0
set {
	beta_program := <bool>$1,
}
`, &IdField{}, userId, joined))
}

func (m *manager) SetUserName(ctx context.Context, userId edgedb.UUID, u *WithNames) error {
	return rewriteEdgedbError(m.c.QuerySingle(ctx, `
update User
filter .id = <uuid>$0
set {
	first_name := <str>$1,
	last_name := <str>$2,
}
`, &IdField{}, userId, u.FirstName, u.LastName))
}

func (m *manager) TrackLogin(ctx context.Context, userId edgedb.UUID, ip string) error {
	now := time.Now().UTC()
	_, err := m.col.UpdateOne(ctx, &IdField{Id: userId}, &bson.M{
		"$inc": LoginCountField{
			LoginCount: 1,
		},
		"$set": withLastLoginInfo{
			LastLoggedInField: LastLoggedInField{
				LastLoggedIn: &now,
			},
			LastLoginIpField: LastLoginIpField{
				LastLoginIp: ip,
			},
		},
		"$push": bson.M{
			"auditLog": bson.M{
				"$each": bson.A{
					AuditLogEntry{
						InitiatorId: userId,
						IpAddress:   ip,
						Operation:   AuditLogOperationLogin,
						Timestamp:   now,
					},
				},
				"$slice": -MaxAuditLogEntries,
			},
		},
	})
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}

func (m *manager) GetEpoch(ctx context.Context, userId edgedb.UUID) (int64, error) {
	var epoch int64
	err := m.c.QuerySingle(ctx, `
select (select User { epoch } filter .id = <uuid>$0).epoch
`, &epoch, userId)
	if err != nil {
		return 0, rewriteEdgedbError(err)
	}
	return epoch, err
}

func (m *manager) GetUsersWithPublicInfo(ctx context.Context, userIds []edgedb.UUID) ([]WithPublicInfo, error) {
	if len(userIds) == 0 {
		return make([]WithPublicInfo, 0), nil
	}
	var users []WithPublicInfo
	c, err := m.col.Find(
		ctx,
		bson.M{
			"_id": bson.M{
				"$in": userIds,
			},
		},
		options.Find().
			SetProjection(getProjection(users)).
			SetBatchSize(int32(len(userIds))),
	)
	if err != nil {
		return nil, rewriteMongoError(err)
	}
	if err = c.All(ctx, &users); err != nil {
		return nil, rewriteMongoError(err)
	}
	return users, nil
}

func (m *manager) GetUsersForBackFilling(ctx context.Context, ids UniqUserIds) (UsersForBackFilling, error) {
	flatIds := make([]edgedb.UUID, 0, len(ids))
	for id := range ids {
		flatIds = append(flatIds, id)
	}
	flatUsers, err := m.GetUsersWithPublicInfo(ctx, flatIds)
	if err != nil {
		return nil, err
	}
	users := make(UsersForBackFilling, len(flatUsers))
	for i := range flatUsers {
		usr := flatUsers[i]
		users[usr.Id] = &usr
	}
	return users, nil
}

func (m *manager) GetUsersForBackFillingNonStandardId(ctx context.Context, ids UniqUserIds) (UsersForBackFillingNonStandardId, error) {
	flatIds := make([]edgedb.UUID, 0, len(ids))
	for id := range ids {
		flatIds = append(flatIds, id)
	}
	flatUsers, err := m.GetUsersWithPublicInfo(ctx, flatIds)
	if err != nil {
		return nil, err
	}
	users := make(UsersForBackFillingNonStandardId, len(flatUsers))
	for _, usr := range flatUsers {
		users[usr.Id] = &WithPublicInfoAndNonStandardId{
			WithPublicInfo: usr,
			IdNoUnderscore: usr.Id,
		}
	}
	return users, nil
}

func rewriteMongoError(err error) error {
	if err == mongo.ErrNoDocuments {
		return &errors.NotFoundError{}
	}
	return err
}

func (m *manager) GetUser(ctx context.Context, userId edgedb.UUID, target interface{}) error {
	var q string
	switch target.(type) {
	case *BetaProgramField:
		q = `
select User { beta_program }
filter .id = <uuid>$0`
	case *WithPublicInfoAndNonStandardId, WithPublicInfo:
		q = `
select User { email: { email }, id, first_name, last_name }
filter .id = <uuid>$0`
	case *ForActivateUserPage:
		q = `
select User { email: { email }, login_count }
filter .id = <uuid>$0`
	case *ForEmailChange:
		q = `
select User { email: { email }, epoch, id }
filter .id = <uuid>$0`
	case *ForPasswordChange:
		q = `
select User {
	email: { email }, epoch, id, first_name, last_name, password_hash
}
filter .id = <uuid>$0`
	case *ForSettingsPage:
		q = `
select User { beta_program, email: { email }, id, first_name, last_name }
filter .id = <uuid>$0`
	default:
		return errors.New("missing query for target")
	}
	if err := m.c.QuerySingle(ctx, q, target, userId); err != nil {
		return rewriteEdgedbError(err)
	}
	return nil
}

func (m *manager) GetUserByEmail(ctx context.Context, email sharedTypes.Email, target interface{}) error {
	var q string
	switch target.(type) {
	case *IdField:
		q = "select User filter .email.email = <str>$0 limit 1"
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
filter .email.email = <str>$0
limit 1`
	default:
		return errors.New("missing query for target")
	}
	if err := m.c.QuerySingle(ctx, q, target, email); err != nil {
		return rewriteEdgedbError(err)
	}
	return nil
}

func (m *manager) getUser(ctx context.Context, filter interface{}, target interface{}) error {
	err := m.col.FindOne(
		ctx,
		filter,
		options.FindOne().SetProjection(getProjection(target)),
	).Decode(target)
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}

func (m *manager) ListProjects(ctx context.Context, userId edgedb.UUID, u interface{}) error {
	err := m.c.QuerySingle(ctx, `
select User {
	email: { email },
	emails: { email },
	first_name,
	id,
	last_name,
	projects: {
		access_ro := ({User} if User in .access_ro else <User>{}),
		access_rw := ({User} if User in .access_rw else <User>{}),
		access_token_ro := ({User} if User in .access_token_ro else <User>{}),
		access_token_rw := ({User} if User in .access_token_rw else <User>{}),
		archived := (User in .archived_by),
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
		trashed := (User in .trashed_by),
	},
	tags: {
		id,
		name,
		projects,
	},
}
filter .id = <uuid>$0
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
filter .id = <uuid>$0
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
		filter .id = <uuid>$0
		set {
			contacts += (
				select detached User {
					@connections := (existing ?? 0) + 1,
					@last_touched := datetime_of_transaction(),
				}
				filter .id = <uuid>$1
			)
		}
	) union (
		update User
		filter .id = <uuid>$1
		set {
			contacts += (
				select detached User {
					@connections := (existing ?? 0) + 1,
					@last_touched := datetime_of_transaction(),
				}
				filter .id = <uuid>$0
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
