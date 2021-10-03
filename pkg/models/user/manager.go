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

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type Manager interface {
	GetEpoch(ctx context.Context, userId primitive.ObjectID) (int64, error)
	GetUser(ctx context.Context, userId primitive.ObjectID, target interface{}) error
	GetUsersWithPublicInfo(ctx context.Context, users []primitive.ObjectID) ([]WithPublicInfo, error)
}

func New(db *mongo.Database) Manager {
	return &manager{
		c: db.Collection("users"),
	}
}

type manager struct {
	c *mongo.Collection
}

func (m *manager) GetEpoch(ctx context.Context, userId primitive.ObjectID) (int64, error) {
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

func rewriteMongoError(err error) error {
	if err == mongo.ErrNoDocuments {
		return &errors.ErrorDocNotFound{}
	}
	return err
}

func (m *manager) GetUser(ctx context.Context, userId primitive.ObjectID, target interface{}) error {
	err := m.c.FindOne(
		ctx,
		IdField{Id: userId},
		options.FindOne().SetProjection(getProjection(target)),
	).Decode(target)
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}
