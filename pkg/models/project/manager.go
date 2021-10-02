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

package project

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Manager interface {
	GetDocMeta(ctx context.Context, projectId, docId primitive.ObjectID) (*Doc, sharedTypes.PathName, error)
	GetJoinProjectDetails(ctx context.Context, projectId, userId primitive.ObjectID) (*JoinProjectViewPrivate, error)
	GetProjectRootFolder(ctx context.Context, projectId primitive.ObjectID) (*Folder, error)
	MarkAsActive(ctx context.Context, projectId primitive.ObjectID) error
	MarkAsInActive(ctx context.Context, projectId primitive.ObjectID) error
	MarkAsOpened(ctx context.Context, projectId primitive.ObjectID) error
	UpdateLastUpdated(ctx context.Context, projectId primitive.ObjectID, at time.Time, by primitive.ObjectID) error
}

func New(db *mongo.Database) (Manager, error) {
	return &manager{
		c: db.Collection("projects"),
	}, nil
}

func rewriteMongoError(err error) error {
	if err == mongo.ErrNoDocuments {
		return &errors.ErrorDocNotFound{}
	}
	return err
}

type manager struct {
	c *mongo.Collection
}

func (m *manager) set(ctx context.Context, projectId primitive.ObjectID, update interface{}) error {
	_, err := m.c.UpdateOne(
		ctx,
		IdField{
			Id: projectId,
		},
		bson.M{
			"$set": update,
		},
	)
	return err
}

func (m *manager) UpdateLastUpdated(ctx context.Context, projectId primitive.ObjectID, at time.Time, by primitive.ObjectID) error {
	v := WithLastUpdatedDetails{}
	v.LastUpdatedAt = at
	v.LastUpdatedBy = by
	_, err := m.c.UpdateOne(
		ctx,
		bson.M{
			"_id": projectId,
			"lastUpdated": bson.M{
				"$gt": at,
			},
		},
		bson.M{
			"$set": v,
		},
	)
	return err
}

func (m *manager) GetDocMeta(ctx context.Context, projectId, docId primitive.ObjectID) (*Doc, sharedTypes.PathName, error) {
	f, err := m.GetProjectRootFolder(ctx, projectId)
	if err != nil {
		return nil, "", errors.Tag(err, "cannot get tree")
	}
	var doc *Doc
	var p sharedTypes.PathName
	err = f.WalkDocs(func(element TreeElement, path sharedTypes.PathName) error {
		if element.GetId() == docId {
			d := element.(Doc)
			doc = &d
			p = path
			return AbortWalk
		}
		return nil
	})
	if err != nil {
		return nil, "", errors.Tag(err, "cannot walk project tree")
	}
	if doc == nil {
		return nil, "", &errors.NotFoundError{}
	}
	return doc, p, nil
}

func (m *manager) GetProjectRootFolder(ctx context.Context, projectId primitive.ObjectID) (*Folder, error) {
	var project WithTree
	err := m.c.FindOne(
		ctx,
		bson.M{
			"_id": projectId,
		},
		options.FindOne().SetProjection(getProjection(project)),
	).Decode(&project)
	if err != nil {
		return nil, rewriteMongoError(err)
	}
	if len(project.RootFolder) != 1 {
		return nil, &errors.ValidationError{
			Msg: fmt.Sprintf(
				"expected rootFolder to have 1 entry, got %d",
				len(project.RootFolder),
			),
		}
	}
	return &project.RootFolder[0], nil
}

func (m *manager) GetJoinProjectDetails(ctx context.Context, projectId, userId primitive.ObjectID) (*JoinProjectViewPrivate, error) {
	var project JoinProjectViewPrivate
	projection := getProjection(project)
	if userId.IsZero() {
		for s := range withMembersProjection {
			delete(projection, s)
		}
	} else {
		limitToUser := bson.M{
			"$elemMatch": bson.M{
				"$eq": userId,
			},
		}
		// These fields are used for an authorization check only, we do not
		//  need to fetch all of them.
		for s := range withTokenMembersProjection {
			projection[s] = limitToUser
		}
	}

	err := m.c.FindOne(
		ctx,
		bson.M{
			"_id": projectId,
		},
		options.FindOne().SetProjection(projection),
	).Decode(&project)
	if err != nil {
		return nil, rewriteMongoError(err)
	}
	return &project, nil
}

func (m *manager) MarkAsActive(ctx context.Context, projectId primitive.ObjectID) error {
	return m.set(ctx, projectId, &ActiveField{Active: true})
}

func (m *manager) MarkAsInActive(ctx context.Context, projectId primitive.ObjectID) error {
	return m.set(ctx, projectId, &ActiveField{Active: false})
}

func (m *manager) MarkAsOpened(ctx context.Context, projectId primitive.ObjectID) error {
	return m.set(ctx, projectId, &LastOpenedField{LastOpened: time.Now()})
}
