// Golang port of Overleaf
// Copyright (C) 2021-2022 Jakob Ackermann <das7pad@outlook.com>
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

package doc

import (
	"context"

	"github.com/edgedb/edgedb-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Manager interface {
	IsDocArchived(
		ctx context.Context,
		projectId edgedb.UUID,
		docId edgedb.UUID,
	) (bool, error)

	IsDocDeleted(
		ctx context.Context,
		projectId edgedb.UUID,
		docId edgedb.UUID,
	) (bool, error)

	CheckDocExists(
		ctx context.Context,
		projectId edgedb.UUID,
		docId edgedb.UUID,
	) error

	GetDocContentsWithFullContext(
		ctx context.Context,
		projectId edgedb.UUID,
		docId edgedb.UUID,
	) (*ContentsWithFullContext, error)

	GetDocForArchiving(
		ctx context.Context,
		projectId edgedb.UUID,
		docId edgedb.UUID,
	) (*ArchiveContext, error)

	GetDocLines(
		ctx context.Context,
		projectId edgedb.UUID,
		docId edgedb.UUID,
	) (sharedTypes.Lines, error)

	PeakDeletedDocNames(
		ctx context.Context,
		projectId edgedb.UUID,
		limit int64,
	) ([]Name, error)

	GetAllRanges(
		ctx context.Context,
		projectId edgedb.UUID,
	) ([]Ranges, error)

	GetAllDocContents(
		ctx context.Context,
		projectId edgedb.UUID,
	) (ContentsCollection, error)

	GetDocIdsForDeletion(
		ctx context.Context,
		projectId edgedb.UUID,
		batchSize int32,
	) (<-chan edgedb.UUID, <-chan error)

	GetDocIdsForArchiving(
		ctx context.Context,
		projectId edgedb.UUID,
		batchSize int32,
	) (<-chan edgedb.UUID, <-chan error)

	GetDocIdsForUnArchiving(
		ctx context.Context,
		projectId edgedb.UUID,
		batchSize int32,
	) (<-chan edgedb.UUID, <-chan error)

	CreateDocWithContent(ctx context.Context, projectId, docId edgedb.UUID, snapshot sharedTypes.Snapshot) error
	CreateDocsWithContent(ctx context.Context, projectId edgedb.UUID, docs []Contents) error

	UpdateDoc(ctx context.Context, projectId, docId edgedb.UUID, update *ForDocUpdate) error
	RestoreArchivedContent(ctx context.Context, projectId, docId edgedb.UUID, contents *ArchiveContents) error

	PatchDocMeta(
		ctx context.Context,
		projectId edgedb.UUID,
		docId edgedb.UUID,
		docMeta Meta,
	) error

	MarkDocAsArchived(
		ctx context.Context,
		projectId edgedb.UUID,
		docId edgedb.UUID,
		revision sharedTypes.Revision,
	) error

	DestroyDoc(
		ctx context.Context,
		projectId edgedb.UUID,
		docId edgedb.UUID,
	) error
}

func New(db *mongo.Database) Manager {
	return &manager{
		c: db.Collection("docs"),
	}
}

const (
	prefetchN                = 100
	ExternalVersionTombstone = -42
)

type manager struct {
	db *mongo.Database
	c  *mongo.Collection
}

func rewriteMongoError(err error) error {
	if err == mongo.ErrNoDocuments {
		return &errors.ErrorDocNotFound{}
	}
	return err
}

func (m *manager) IsDocArchived(ctx context.Context, projectId edgedb.UUID, docId edgedb.UUID) (bool, error) {
	var doc InS3Field
	err := m.c.FindOne(
		ctx,
		docFilter(projectId, docId),
		options.FindOne().SetProjection(getProjection(doc)),
	).Decode(&doc)
	if err != nil {
		return false, rewriteMongoError(err)
	}
	return doc.IsArchived(), nil
}

func (m *manager) IsDocDeleted(ctx context.Context, projectId edgedb.UUID, docId edgedb.UUID) (bool, error) {
	var doc DeletedField
	err := m.c.FindOne(
		ctx,
		docFilter(projectId, docId),
		options.FindOne().SetProjection(getProjection(doc)),
	).Decode(&doc)
	if err != nil {
		return false, rewriteMongoError(err)
	}
	return doc.Deleted, nil
}

func (m *manager) CheckDocExists(ctx context.Context, projectId edgedb.UUID, docId edgedb.UUID) error {
	err := m.c.FindOne(
		ctx,
		docFilter(projectId, docId),
		options.FindOne().SetProjection(docIdFieldProjection),
	).Err()
	return rewriteMongoError(err)
}

func (m *manager) GetDocContentsWithFullContext(ctx context.Context, projectId edgedb.UUID, docId edgedb.UUID) (*ContentsWithFullContext, error) {
	var doc ContentsWithFullContext
	err := m.c.FindOne(
		ctx,
		docFilter(projectId, docId),
		options.FindOne().SetProjection(getProjection(doc)),
	).Decode(&doc)
	if err != nil {
		return nil, rewriteMongoError(err)
	}
	if err = doc.Validate(); err != nil {
		return nil, err
	}
	return &doc, nil
}

func (m *manager) GetDocForArchiving(ctx context.Context, projectId edgedb.UUID, docId edgedb.UUID) (*ArchiveContext, error) {
	var doc ArchiveContext
	err := m.c.FindOne(
		ctx,
		docFilter(projectId, docId),
		options.FindOne().SetProjection(getProjection(doc)),
	).Decode(&doc)
	if err != nil {
		return nil, rewriteMongoError(err)
	}
	if err = doc.Validate(); err != nil {
		return nil, err
	}

	return &doc, nil
}

func (m *manager) GetDocLines(ctx context.Context, projectId edgedb.UUID, docId edgedb.UUID) (sharedTypes.Lines, error) {
	var doc Lines
	err := m.c.FindOne(
		ctx,
		docFilter(projectId, docId),
		options.FindOne().SetProjection(getProjection(doc)),
	).Decode(&doc)
	if err != nil {
		return nil, rewriteMongoError(err)
	}
	if err = doc.Validate(); err != nil {
		return nil, err
	}

	return doc.Lines, nil
}

func (m *manager) PeakDeletedDocNames(ctx context.Context, projectId edgedb.UUID, limit int64) ([]Name, error) {
	docs := make(NameCollection, 0)
	res, err := m.c.Find(
		ctx,
		projectFilterDeleted(projectId),
		options.Find().
			SetProjection(getProjection(docs)).
			SetSort(bson.M{
				"deletedAt": -1,
			}).
			SetLimit(limit).
			SetBatchSize(int32(limit)),
	)
	if err != nil {
		return nil, rewriteMongoError(err)
	}
	err = res.All(ctx, &docs)
	if err != nil {
		return nil, rewriteMongoError(err)
	}
	return docs, nil
}

func (m *manager) GetAllRanges(ctx context.Context, projectId edgedb.UUID) ([]Ranges, error) {
	docs := make(RangesCollection, 0)
	res, err := m.c.Find(
		ctx,
		projectFilterNonDeleted(projectId),
		options.Find().
			SetProjection(getProjection(docs)).
			SetBatchSize(prefetchN),
	)
	if err != nil {
		return nil, rewriteMongoError(err)
	}
	err = res.All(ctx, &docs)
	if err != nil {
		return nil, rewriteMongoError(err)
	}

	if err = docs.Validate(); err != nil {
		return nil, err
	}
	return docs, nil
}

func (m *manager) GetAllDocContents(ctx context.Context, projectId edgedb.UUID) (ContentsCollection, error) {
	docs := make(ContentsCollection, 0)
	res, err := m.c.Find(
		ctx,
		projectFilterNonDeleted(projectId),
		options.Find().
			SetProjection(getProjection(docs)).
			SetBatchSize(prefetchN),
	)
	if err != nil {
		return nil, rewriteMongoError(err)
	}
	err = res.All(ctx, &docs)
	if err != nil {
		return nil, rewriteMongoError(err)
	}
	if err = docs.Validate(); err != nil {
		return nil, err
	}
	return docs, nil
}

func (m *manager) streamDocIds(ctx context.Context, filter bson.M, batchSize int32) (<-chan edgedb.UUID, <-chan error) {
	ids := make(chan edgedb.UUID)
	errs := make(chan error)

	go func() {
		defer close(ids)
		defer close(errs)

		cursor, err := m.c.Find(
			ctx,
			filter,
			options.Find().
				SetBatchSize(batchSize).
				SetProjection(docIdFieldProjection),
		)
		if err != nil {
			errs <- rewriteMongoError(err)
			return
		}

		for cursor.Next(ctx) {
			var doc IdField
			if err = cursor.Decode(&doc); err != nil {
				errs <- rewriteMongoError(err)
				break
			}
			ids <- doc.Id
		}

		if err = cursor.Err(); err != nil {
			errs <- rewriteMongoError(err)
		}
		if err = cursor.Close(ctx); err != nil {
			errs <- rewriteMongoError(err)
		}
	}()
	return ids, errs
}

func (m *manager) GetDocIdsForDeletion(ctx context.Context, projectId edgedb.UUID, batchSize int32) (<-chan edgedb.UUID, <-chan error) {
	return m.streamDocIds(
		ctx,
		projectFilterAllDocs(projectId),
		batchSize,
	)
}

func (m *manager) GetDocIdsForArchiving(ctx context.Context, projectId edgedb.UUID, batchSize int32) (<-chan edgedb.UUID, <-chan error) {
	return m.streamDocIds(
		ctx,
		projectFilterNonArchivedDocs(projectId),
		batchSize,
	)
}

func (m *manager) GetDocIdsForUnArchiving(ctx context.Context, projectId edgedb.UUID, batchSize int32) (<-chan edgedb.UUID, <-chan error) {
	return m.streamDocIds(
		ctx,
		projectFilterNeedsUnArchiving(projectId),
		batchSize,
	)
}

func (m *manager) CreateDocWithContent(ctx context.Context, projectId, docId edgedb.UUID, snapshot sharedTypes.Snapshot) error {
	_, err := m.c.InsertOne(ctx, &forInsertion{
		IdField:        IdField{Id: docId},
		ProjectIdField: ProjectIdField{ProjectId: projectId},
		LinesField:     LinesField{Lines: snapshot.ToLines()},
		RevisionField:  RevisionField{Revision: 0},
		VersionField:   VersionField{Version: 0},
	})
	if err != nil {
		return errors.Tag(err, "cannot insert doc")
	}
	return nil
}

func (m *manager) CreateDocsWithContent(ctx context.Context, projectId edgedb.UUID, docs []Contents) error {
	docContents := make([]interface{}, len(docs))
	for i, doc := range docs {
		docContents[i] = &forInsertion{
			IdField:        doc.IdField,
			LinesField:     doc.LinesField,
			ProjectIdField: ProjectIdField{ProjectId: projectId},
			RevisionField:  RevisionField{Revision: 0},
			VersionField:   VersionField{Version: 0},
		}
	}
	if _, err := m.c.InsertMany(ctx, docContents); err != nil {
		return errors.Tag(err, "cannot insert doc contents")
	}
	return nil
}

func (m *manager) UpdateDoc(ctx context.Context, projectId, docId edgedb.UUID, update *ForDocUpdate) error {
	_, err := m.c.UpdateOne(
		ctx,
		docFilter(projectId, docId),
		bson.M{
			"$set":   update,
			"$inc":   RevisionField{Revision: 1},
			"$unset": InS3Field{InS3: true},
		},
	)
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}

func (m *manager) RestoreArchivedContent(ctx context.Context, projectId, docId edgedb.UUID, contents *ArchiveContents) error {
	_, err := m.c.UpdateOne(
		ctx,
		docFilterInS3(projectId, docId),
		bson.M{
			"$set":   contents,
			"$inc":   RevisionField{Revision: 1},
			"$unset": InS3Field{InS3: true},
		},
	)
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}

func (m *manager) PatchDocMeta(ctx context.Context, projectId edgedb.UUID, docId edgedb.UUID, meta Meta) error {
	_, err := m.c.UpdateOne(
		ctx,
		docFilter(projectId, docId),
		bson.M{
			"$set": meta,
		},
	)
	return err
}

func (m *manager) MarkDocAsArchived(ctx context.Context, projectId edgedb.UUID, docId edgedb.UUID, revision sharedTypes.Revision) error {
	filter := docFilterWithRevision(projectId, docId, revision)
	_, err := m.c.UpdateOne(
		ctx,
		filter,
		bson.M{
			"$set":   InS3Field{InS3: true},
			"$unset": docArchiveContentsFields,
		},
	)
	return rewriteMongoError(err)
}

func (m *manager) DestroyDoc(ctx context.Context, projectId edgedb.UUID, docId edgedb.UUID) error {
	if _, err := m.c.DeleteOne(ctx, docFilter(projectId, docId)); err != nil {
		return rewriteMongoError(err)
	}
	return nil
}
