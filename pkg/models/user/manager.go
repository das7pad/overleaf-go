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
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Manager interface {
	Delete(ctx context.Context, userId primitive.ObjectID, epoch int64) error
	TrackClearSessions(ctx context.Context, userId primitive.ObjectID, ip string, info interface{}) error
	BumpEpoch(ctx context.Context, userId primitive.ObjectID) error
	GetEpoch(ctx context.Context, userId primitive.ObjectID) (int64, error)
	GetProjectMembers(ctx context.Context, readOnly, readAndWrite []primitive.ObjectID) ([]AsProjectMember, error)
	GetUser(ctx context.Context, userId primitive.ObjectID, target interface{}) error
	GetUserByEmail(ctx context.Context, email sharedTypes.Email, target interface{}) error
	GetUsersWithPublicInfo(ctx context.Context, users []primitive.ObjectID) ([]WithPublicInfo, error)
	GetUsersForBackFilling(ctx context.Context, ids UniqUserIds) (UsersForBackFilling, error)
	GetUsersForBackFillingNonStandardId(ctx context.Context, ids UniqUserIds) (UsersForBackFillingNonStandardId, error)
	SetBetaProgram(ctx context.Context, userId primitive.ObjectID, joined bool) error
	UpdateEditorConfig(ctx context.Context, userId primitive.ObjectID, config EditorConfig) error
	TrackLogin(ctx context.Context, userId primitive.ObjectID, ip string) error
	ChangeEmailAddress(ctx context.Context, change *ForEmailChange, ip string, newEmail sharedTypes.Email) error
	SetUserName(ctx context.Context, userId primitive.ObjectID, u *WithNames) error
}

func New(db *mongo.Database) Manager {
	return &manager{
		c: db.Collection("users"),
	}
}

type manager struct {
	c *mongo.Collection
}

func (m *manager) UpdateEditorConfig(ctx context.Context, userId primitive.ObjectID, editorConfig EditorConfig) error {
	q := &IdField{Id: userId}
	u := bson.M{
		"$set": &EditorConfigField{
			EditorConfig: editorConfig,
		},
	}
	_, err := m.c.UpdateOne(ctx, q, u)
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}

var ErrEpochChanged = &errors.InvalidStateError{Msg: "user epoch changed"}

func (m *manager) Delete(ctx context.Context, userId primitive.ObjectID, epoch int64) error {
	q := &withIdAndEpoch{
		IdField: IdField{
			Id: userId,
		},
		EpochField: EpochField{
			Epoch: epoch,
		},
	}
	r, err := m.c.DeleteOne(ctx, q)
	if err != nil {
		return rewriteMongoError(err)
	}
	if r.DeletedCount != 1 {
		return ErrEpochChanged
	}
	return nil
}

func (m *manager) TrackClearSessions(ctx context.Context, userId primitive.ObjectID, ip string, info interface{}) error {
	now := time.Now().UTC()
	_, err := m.c.UpdateOne(ctx, &IdField{Id: userId}, &bson.M{
		"$inc": EpochField{Epoch: 1},
		"$push": bson.M{
			"auditLog": bson.M{
				"$each": bson.A{
					AuditLogEntry{
						Info:        info,
						InitiatorId: userId,
						IpAddress:   ip,
						Operation:   "clear-sessions",
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
	r, err := m.c.UpdateOne(ctx, q, bson.M{
		"$set": withEmailFields{
			EmailField{
				Email: newEmail,
			},
			EmailsField{
				Emails: []EmailDetails{
					{
						Id:               primitive.NewObjectID(),
						CreatedAt:        now,
						Email:            newEmail,
						ReversedHostname: newEmail.ReversedHostname(),
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
						Operation:   "change-primary-email",
						Timestamp:   now,
					},
				},
				"$slice": -MaxAuditLogEntries,
			},
		},
	})
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return &errors.InvalidStateError{Msg: "email already registered"}
		}
		return rewriteMongoError(err)
	}
	if r.ModifiedCount != 1 {
		return ErrEpochChanged
	}
	return nil
}

const (
	AnonymousUserEpoch = 0
	MaxAuditLogEntries = 200
)

func (m *manager) BumpEpoch(ctx context.Context, userId primitive.ObjectID) error {
	_, err := m.c.UpdateOne(ctx, &IdField{Id: userId}, &bson.M{
		"$inc": &EpochField{Epoch: 1},
	})
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}

func (m *manager) SetBetaProgram(ctx context.Context, userId primitive.ObjectID, joined bool) error {
	_, err := m.c.UpdateOne(ctx, &IdField{Id: userId}, &bson.M{
		"$set": &BetaProgramField{BetaProgram: joined},
	})
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}

func (m *manager) SetUserName(ctx context.Context, userId primitive.ObjectID, u *WithNames) error {
	_, err := m.c.UpdateOne(ctx, &IdField{Id: userId}, &bson.M{
		"$set": u,
	})
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}

func (m *manager) TrackLogin(ctx context.Context, userId primitive.ObjectID, ip string) error {
	now := time.Now().UTC()
	_, err := m.c.UpdateOne(ctx, &IdField{Id: userId}, &bson.M{
		"$inc": LoginCountField{
			LoginCount: 1,
		},
		"$set": withLastLoginInfo{
			LastLoggedInField: LastLoggedInField{
				LastLoggedIn: now,
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
						Operation:   "login",
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

func (m *manager) GetEpoch(ctx context.Context, userId primitive.ObjectID) (int64, error) {
	if userId.IsZero() {
		return AnonymousUserEpoch, nil
	}
	p := &EpochField{}
	err := m.GetUser(ctx, userId, p)
	return p.Epoch, err
}

func (m *manager) GetUsersWithPublicInfo(ctx context.Context, userIds []primitive.ObjectID) ([]WithPublicInfo, error) {
	if len(userIds) == 0 {
		return make([]WithPublicInfo, 0), nil
	}
	var users []WithPublicInfo
	c, err := m.c.Find(
		ctx,
		bson.M{
			"_id": bson.M{
				"$in": userIds,
			},
		},
		options.Find().SetProjection(getProjection(users)),
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
	flatIds := make([]primitive.ObjectID, 0, len(ids))
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
	flatIds := make([]primitive.ObjectID, 0, len(ids))
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

func (m *manager) GetProjectMembers(ctx context.Context, readOnly, readAndWrite []primitive.ObjectID) ([]AsProjectMember, error) {
	ids := make(UniqUserIds, len(readOnly)+len(readAndWrite))
	for _, id := range readOnly {
		ids[id] = true
	}
	for _, id := range readAndWrite {
		ids[id] = true
	}
	users, err := m.GetUsersForBackFilling(ctx, ids)
	if err != nil {
		return nil, err
	}
	members := make([]AsProjectMember, 0, len(users))
	for _, id := range readOnly {
		u, exists := users[id]
		if !exists {
			continue
		}
		members = append(members, AsProjectMember{
			WithPublicInfo: u,
			PrivilegeLevel: sharedTypes.PrivilegeLevelReadOnly,
		})
	}
	for _, id := range readAndWrite {
		u, exists := users[id]
		if !exists {
			continue
		}
		members = append(members, AsProjectMember{
			WithPublicInfo: u,
			PrivilegeLevel: sharedTypes.PrivilegeLevelReadAndWrite,
		})
	}
	return members, nil
}

func rewriteMongoError(err error) error {
	if err == mongo.ErrNoDocuments {
		return &errors.NotFoundError{}
	}
	return err
}

func (m *manager) GetUser(ctx context.Context, userId primitive.ObjectID, target interface{}) error {
	return m.getUser(ctx, &IdField{Id: userId}, target)
}

func (m *manager) GetUserByEmail(ctx context.Context, email sharedTypes.Email, target interface{}) error {
	return m.getUser(ctx, &EmailField{Email: email}, target)
}

func (m *manager) getUser(ctx context.Context, filter interface{}, target interface{}) error {
	err := m.c.FindOne(
		ctx,
		filter,
		options.FindOne().SetProjection(getProjection(target)),
	).Decode(target)
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}
