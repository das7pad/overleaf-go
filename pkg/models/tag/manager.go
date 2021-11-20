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

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type Manager interface {
	AddProject(ctx context.Context, userId, tagId, projectId primitive.ObjectID) error
	Delete(ctx context.Context, userId, tagId primitive.ObjectID) error
	EnsureExists(ctx context.Context, userId primitive.ObjectID, name string) (*Full, error)
	GetAll(ctx context.Context, userId primitive.ObjectID) ([]Full, error)
	RemoveProject(ctx context.Context, userId, tagId, projectId primitive.ObjectID) error
	RemoveProjectBulk(ctx context.Context, userId, projectId primitive.ObjectID) error
	Rename(ctx context.Context, userId, tagId primitive.ObjectID, newName string) error
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
		return &errors.NotFoundError{}
	}
	return err
}

func filterByUserAndTagName(userId primitive.ObjectID, name string) interface{} {
	q := &userIdAndTagName{}
	q.Name = name
	q.UserId = userId.Hex()
	return q
}

func filterByUserAndTagId(userId, tagId primitive.ObjectID) interface{} {
	q := &userIdAndTagId{}
	q.Id = tagId
	q.UserId = userId.Hex()
	return q
}

func (m *manager) AddProject(ctx context.Context, userId, tagId, projectId primitive.ObjectID) error {
	q := filterByUserAndTagId(userId, tagId)
	_, err := m.c.UpdateOne(ctx, q, &bson.M{
		"$addToSet": bson.M{
			"project_ids": projectId,
		},
	})
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}

func (m *manager) Delete(ctx context.Context, userId, tagId primitive.ObjectID) error {
	q := filterByUserAndTagId(userId, tagId)
	_, err := m.c.DeleteOne(ctx, q)
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}

var initWithEmptyListOfProjects = &bson.M{
	"$setOnInsert": &ProjectIdsField{
		ProjectIds: make([]primitive.ObjectID, 0),
	},
}

func (m *manager) EnsureExists(ctx context.Context, userId primitive.ObjectID, name string) (*Full, error) {
	q := filterByUserAndTagName(userId, name)
	t := &Full{}
	err := m.c.FindOneAndUpdate(
		ctx,
		q,
		initWithEmptyListOfProjects,
		options.FindOneAndUpdate().
			SetUpsert(true).
			SetReturnDocument(options.After),
	).Decode(t)
	if err != nil {
		return nil, rewriteMongoError(err)
	}
	return t, nil
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

func (m *manager) RemoveProject(ctx context.Context, userId, tagId, projectId primitive.ObjectID) error {
	q := filterByUserAndTagId(userId, tagId)
	_, err := m.c.UpdateOne(ctx, q, &bson.M{
		"$pull": bson.M{
			"project_ids": projectId,
		},
	})
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}

func (m *manager) RemoveProjectBulk(ctx context.Context, userId, projectId primitive.ObjectID) error {
	q := &UserIdField{UserId: userId.Hex()}
	_, err := m.c.UpdateMany(ctx, q, &bson.M{
		"$pull": bson.M{
			"project_ids": projectId,
		},
	})
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}

func (m *manager) Rename(ctx context.Context, userId, tagId primitive.ObjectID, newName string) error {
	q := filterByUserAndTagId(userId, tagId)
	_, err := m.c.UpdateOne(ctx, q, &bson.M{
		"$set": NameField{Name: newName},
	})
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}
