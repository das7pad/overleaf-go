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

package systemMessage

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type Manager interface {
	Create(ctx context.Context, content ContentField) error
	DeleteAll(ctx context.Context) error
	GetAll(ctx context.Context) ([]Full, error)
}

func New(db *mongo.Database) Manager {
	return &manager{
		c: db.Collection("systemmessages"),
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

func (m *manager) Create(ctx context.Context, content ContentField) error {
	_, err := m.c.InsertOne(ctx, content)
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}

func (m *manager) DeleteAll(ctx context.Context) error {
	if _, err := m.c.DeleteMany(ctx, bson.M{}); err != nil {
		return rewriteMongoError(err)
	}
	return nil
}

func (m *manager) GetAll(ctx context.Context) ([]Full, error) {
	messages := make([]Full, 0)
	r, err := m.c.Find(
		ctx,
		bson.M{},
		options.Find().SetProjection(getProjection(messages)),
	)
	if err != nil {
		return nil, rewriteMongoError(err)
	}
	if err = r.All(ctx, &messages); err != nil {
		return nil, rewriteMongoError(err)
	}
	return messages, nil
}
