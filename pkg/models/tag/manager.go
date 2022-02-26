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

	"github.com/edgedb/edgedb-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type Manager interface {
	AddProject(ctx context.Context, userId, tagId, projectId edgedb.UUID) error
	Delete(ctx context.Context, userId, tagId edgedb.UUID) error
	DeleteForUser(ctx context.Context, userId edgedb.UUID) error
	EnsureExists(ctx context.Context, userId edgedb.UUID, name string) (*Full, error)
	GetAll(ctx context.Context, userId edgedb.UUID) ([]Full, error)
	RemoveProjectFromTag(ctx context.Context, userId, tagId, projectId edgedb.UUID) error
	RemoveProjectForAllUsers(ctx context.Context, userIds []edgedb.UUID, projectId edgedb.UUID) error
	RemoveProjectForUser(ctx context.Context, userId, projectId edgedb.UUID) error
	Rename(ctx context.Context, userId, tagId edgedb.UUID, newName string) error
}

func New(db *mongo.Database) Manager {
	return &manager{
		c: db.Collection("tags"),
	}
}

const (
	prefetchN = 100
)

type manager struct {
	c *mongo.Collection
}

func rewriteMongoError(err error) error {
	if err == mongo.ErrNoDocuments {
		return &errors.NotFoundError{}
	}
	return err
}

func filterByUserAndTagName(userId edgedb.UUID, name string) interface{} {
	q := &userIdAndTagName{}
	q.Name = name
	q.UserId = userId.String()
	return q
}

func filterByUserAndTagId(userId, tagId edgedb.UUID) interface{} {
	q := &userIdAndTagId{}
	q.Id = tagId
	q.UserId = userId.String()
	return q
}

func (m *manager) AddProject(ctx context.Context, userId, tagId, projectId edgedb.UUID) error {
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

func (m *manager) Delete(ctx context.Context, userId, tagId edgedb.UUID) error {
	q := filterByUserAndTagId(userId, tagId)
	_, err := m.c.DeleteOne(ctx, q)
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}

func (m *manager) DeleteForUser(ctx context.Context, userId edgedb.UUID) error {
	q := &UserIdField{UserId: userId.String()}
	_, err := m.c.DeleteMany(ctx, q)
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}

var initWithEmptyListOfProjects = &bson.M{
	"$setOnInsert": &ProjectIdsField{
		ProjectIds: make([]edgedb.UUID, 0),
	},
}

func (m *manager) EnsureExists(ctx context.Context, userId edgedb.UUID, name string) (*Full, error) {
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

func (m *manager) GetAll(ctx context.Context, userId edgedb.UUID) ([]Full, error) {
	tags := make([]Full, 0)
	r, err := m.c.Find(
		ctx,
		UserIdField{UserId: userId.String()},
		options.Find().
			SetProjection(getProjection(tags)).
			SetBatchSize(prefetchN),
	)
	if err != nil {
		return nil, rewriteMongoError(err)
	}
	if err = r.All(ctx, &tags); err != nil {
		return nil, rewriteMongoError(err)
	}
	return tags, nil
}

func (m *manager) RemoveProjectFromTag(ctx context.Context, userId, tagId, projectId edgedb.UUID) error {
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

func (m *manager) RemoveProjectForAllUsers(ctx context.Context, userIds []edgedb.UUID, projectId edgedb.UUID) error {
	userIdsHex := make([]string, len(userIds))
	for i, id := range userIds {
		userIdsHex[i] = id.String()
	}
	q := bson.M{
		"user_id": bson.M{
			"$in": userIdsHex,
		},
	}
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

func (m *manager) RemoveProjectForUser(ctx context.Context, userId, projectId edgedb.UUID) error {
	q := &UserIdField{UserId: userId.String()}
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

func (m *manager) Rename(ctx context.Context, userId, tagId edgedb.UUID, newName string) error {
	q := filterByUserAndTagId(userId, tagId)
	_, err := m.c.UpdateOne(ctx, q, &bson.M{
		"$set": NameField{Name: newName},
	})
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}
