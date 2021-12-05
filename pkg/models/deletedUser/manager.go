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

package deletedUser

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/user"
)

type Manager interface {
	Create(ctx context.Context, deletion *user.ForDeletion, userId primitive.ObjectID, ipAddress string) error
}

func New(db *mongo.Database) Manager {
	return &manager{
		c: db.Collection("deletedUsers"),
	}
}

type manager struct {
	c *mongo.Collection
}

func rewriteMongoError(err error) error {
	if err == mongo.ErrNoDocuments {
		return &errors.NotFoundError{}
	}
	return err
}

func (m *manager) Create(ctx context.Context, u *user.ForDeletion, userId primitive.ObjectID, ipAddress string) error {
	entry := &Full{
		UserField: UserField{User: u},
		DeleterDataField: DeleterDataField{
			DeleterData: DeleterData{
				DeletedAt:               time.Now().UTC(),
				DeleterId:               userId,
				DeleterIpAddress:        ipAddress,
				DeletedUserId:           u.Id,
				DeletedUserLastLoggedIn: u.LastLoggedIn,
				DeletedUserSignUpDate:   u.SignUpDate,
				DeletedUserLoginCount:   u.LoginCount,
				DeletedUserReferralId:   u.ReferralId,
			},
		},
	}
	_, err := m.c.InsertOne(ctx, entry)
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}
