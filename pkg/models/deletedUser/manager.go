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

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/user"
)

type Manager interface {
	Create(ctx context.Context, deletion *user.ForDeletion, userId primitive.ObjectID, ipAddress string) error
	Expire(ctx context.Context, projectId primitive.ObjectID) error
	GetExpired(ctx context.Context, age time.Duration) (<-chan primitive.ObjectID, error)
}

func New(db *mongo.Database) Manager {
	c := db.Collection("deletedUsers")
	cSlow := db.Collection("deletedUsers", options.Collection().
		SetReadPreference(readpref.SecondaryPreferred()),
	)
	return &manager{
		c:     c,
		cSlow: cSlow,
	}
}

type manager struct {
	c     *mongo.Collection
	cSlow *mongo.Collection
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
				DeleterDataDeletedUserIdField: DeleterDataDeletedUserIdField{
					DeletedUserId: u.Id,
				},
				DeletedAt:               time.Now().UTC(),
				DeleterId:               userId,
				DeleterIpAddress:        ipAddress,
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

func (m *manager) Expire(ctx context.Context, userId primitive.ObjectID) error {
	q := bson.M{
		"deleterData.deletedUserId": userId,
	}
	u := bson.M{
		"$set": bson.M{
			"user":                         nil,
			"deleterData.deleterIpAddress": "",
		},
	}
	_, err := m.c.UpdateOne(ctx, q, u)
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}

const bufferSize = 10

func (m *manager) GetExpired(ctx context.Context, age time.Duration) (<-chan primitive.ObjectID, error) {
	q := bson.M{
		"deleterData.deletedAt": bson.M{
			"$lt": time.Now().UTC().Add(-age),
		},
		"user": bson.M{
			"$ne": nil,
		},
	}
	projection := bson.M{
		"deleterData.deletedUserId": 1,
	}
	r, errFind := m.cSlow.Find(
		ctx, q, options.Find().
			SetProjection(projection).
			SetBatchSize(bufferSize),
	)
	if errFind != nil {
		return nil, rewriteMongoError(errFind)
	}
	queue := make(chan primitive.ObjectID, bufferSize)

	// Peek once into the batch, then ignore any errors during background
	//  streaming.
	if !r.Next(ctx) {
		close(queue)
		if err := r.Err(); err != nil {
			return nil, err
		}
		return queue, nil
	}
	dp := &forListing{}
	if err := r.Decode(dp); err != nil {
		close(queue)
		return nil, err
	}

	go func() {
		defer close(queue)
		queue <- dp.DeleterData.DeletedUserId
		for r.Next(ctx) {
			if err := r.Decode(dp); err != nil {
				return
			}
			queue <- dp.DeleterData.DeletedUserId
		}
	}()
	return queue, nil
}
