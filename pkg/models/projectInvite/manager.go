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

package projectInvite

import (
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type Manager interface {
	Delete(ctx context.Context, projectId, inviteId primitive.ObjectID) error
	Get(ctx context.Context, projectId primitive.ObjectID, token Token) (*WithoutToken, error)
	GetAllForProject(ctx context.Context, projectId primitive.ObjectID) ([]*WithoutToken, error)
}

func New(db *mongo.Database) Manager {
	return &manager{
		c: db.Collection("projectInvites"),
	}
}

func rewriteMongoError(err error) error {
	if err == mongo.ErrNoDocuments {
		return &errors.NotFoundError{}
	}
	return err
}

type manager struct {
	c *mongo.Collection
}

func (m *manager) Delete(ctx context.Context, projectId, inviteId primitive.ObjectID) error {
	q := projectIdAndInviteId{}
	q.Id = inviteId
	q.ProjectId = projectId
	r, err := m.c.DeleteOne(ctx, q)
	if err != nil {
		return rewriteMongoError(err)
	}
	if r.DeletedCount != 1 {
		return &errors.NotFoundError{}
	}
	return nil
}

func (m *manager) Get(ctx context.Context, projectId primitive.ObjectID, token Token) (*WithoutToken, error) {
	q := projectIdAndToken{}
	q.ProjectId = projectId
	q.Token = token

	pi := &WithoutToken{}
	err := m.c.FindOne(
		ctx, q, options.FindOne().SetProjection(getProjection(pi)),
	).Decode(pi)
	if err != nil {
		return nil, rewriteMongoError(err)
	}
	return pi, nil
}

func (m *manager) GetAllForProject(ctx context.Context, projectId primitive.ObjectID) ([]*WithoutToken, error) {
	q := ProjectIdField{ProjectId: projectId}

	invites := make([]*WithoutToken, 0)
	r, err := m.c.Find(
		ctx, q, options.Find().SetProjection(getProjection(invites)),
	)
	if err != nil {
		return nil, rewriteMongoError(err)
	}
	if err = r.All(ctx, &invites); err != nil {
		return nil, rewriteMongoError(err)
	}
	return invites, nil
}
