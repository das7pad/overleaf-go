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

package tag

import (
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type Manager interface {
	GetAll(ctx context.Context, userId primitive.ObjectID) ([]Full, error)
}

func New(db *mongo.Database) Manager {
	return &manager{
		c: db.Collection("tags"),
	}
}

type manager struct {
	c *mongo.Collection
}

func rewriteMongoError(err error) error {
	if err == mongo.ErrNoDocuments {
		return &errors.ErrorDocNotFound{}
	}
	return err
}

func (m *manager) GetAll(ctx context.Context, userId primitive.ObjectID) ([]Full, error) {
	tags := make([]Full, 0)
	r, err := m.c.Find(
		ctx,
		UserIdField{UserId: userId.Hex()},
		options.Find().SetProjection(getProjection(tags)),
	)
	if err != nil {
		return nil, rewriteMongoError(err)
	}
	if err = r.All(ctx, &tags); err != nil {
		return nil, rewriteMongoError(err)
	}
	return tags, nil
}
