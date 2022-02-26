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

package deletedFile

import (
	"context"
	"time"

	"github.com/edgedb/edgedb-go"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
)

type Manager interface {
	Create(ctx context.Context, projectId edgedb.UUID, fileRef *project.FileRef) error
	CreateBulk(ctx context.Context, projectId edgedb.UUID, fileRefs []*project.FileRef) error
	DeleteBulk(ctx context.Context, projectId edgedb.UUID) error
}

func New(db *mongo.Database) Manager {
	return &manager{
		c: db.Collection("deletedFiles"),
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

func (m *manager) Create(ctx context.Context, projectId edgedb.UUID, fileRef *project.FileRef) error {
	entry := &Full{}
	entry.DeletedAt = time.Now().UTC()
	entry.FileRef = *fileRef
	entry.ProjectId = projectId
	_, err := m.c.InsertOne(ctx, entry)
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}

func (m *manager) CreateBulk(ctx context.Context, projectId edgedb.UUID, fileRefs []*project.FileRef) error {
	items := make([]interface{}, len(fileRefs))
	now := time.Now().UTC()
	for i := range fileRefs {
		entry := &Full{}
		entry.DeletedAt = now
		entry.FileRef = *fileRefs[i]
		entry.ProjectId = projectId
		items[i] = entry
	}
	_, err := m.c.InsertMany(ctx, items)
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}

func (m *manager) DeleteBulk(ctx context.Context, projectId edgedb.UUID) error {
	q := &ProjectIdField{ProjectId: projectId}
	if _, err := m.c.DeleteMany(ctx, q); err != nil {
		return rewriteMongoError(err)
	}
	return nil
}
