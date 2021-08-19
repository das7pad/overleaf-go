// Golang port of the Overleaf docstore service
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

package docs

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/docstore/pkg/models"
)

type Manager interface {
	IsDocArchived(
		ctx context.Context,
		projectId primitive.ObjectID,
		docId primitive.ObjectID,
	) (bool, error)

	IsDocDeleted(
		ctx context.Context,
		projectId primitive.ObjectID,
		docId primitive.ObjectID,
	) (bool, error)

	CheckDocExists(
		ctx context.Context,
		projectId primitive.ObjectID,
		docId primitive.ObjectID,
	) error

	GetDocContentsWithFullContext(
		ctx context.Context,
		projectId primitive.ObjectID,
		docId primitive.ObjectID,
	) (*models.DocContentsWithFullContext, error)

	GetDocForArchiving(
		ctx context.Context,
		projectId primitive.ObjectID,
		docId primitive.ObjectID,
	) (*models.DocArchiveContext, error)

	GetDocLines(
		ctx context.Context,
		projectId primitive.ObjectID,
		docId primitive.ObjectID,
	) (sharedTypes.Lines, error)

	PeakDeletedDocNames(
		ctx context.Context,
		projectId primitive.ObjectID,
		limit int64,
	) ([]models.DocName, error)

	GetAllRanges(
		ctx context.Context,
		projectId primitive.ObjectID,
	) ([]models.DocRanges, error)

	GetAllDocContents(
		ctx context.Context,
		projectId primitive.ObjectID,
	) ([]models.DocContents, error)

	GetDocIdsForDeletion(
		ctx context.Context,
		projectId primitive.ObjectID,
		batchSize int32,
	) (<-chan primitive.ObjectID, <-chan error)

	GetDocIdsForArchiving(
		ctx context.Context,
		projectId primitive.ObjectID,
		batchSize int32,
	) (<-chan primitive.ObjectID, <-chan error)

	GetDocIdsForUnArchiving(
		ctx context.Context,
		projectId primitive.ObjectID,
		batchSize int32,
	) (<-chan primitive.ObjectID, <-chan error)

	UpsertDoc(
		ctx context.Context,
		projectId primitive.ObjectID,
		docId primitive.ObjectID,
		lines sharedTypes.Lines,
		ranges sharedTypes.Ranges,
	) (sharedTypes.Revision, error)

	SetDocVersion(
		ctx context.Context,
		docId primitive.ObjectID,
		version sharedTypes.Version,
	) error

	PatchDocMeta(
		ctx context.Context,
		projectId primitive.ObjectID,
		docId primitive.ObjectID,
		docMeta models.DocMeta,
	) error

	MarkDocAsArchived(
		ctx context.Context,
		projectId primitive.ObjectID,
		docId primitive.ObjectID,
		revision sharedTypes.Revision,
	) error

	DestroyDoc(
		ctx context.Context,
		projectId primitive.ObjectID,
		docId primitive.ObjectID,
	) error
}

func New(db *mongo.Database) Manager {
	return &manager{
		cDocs:   db.Collection("docs"),
		cDocOps: db.Collection("docOps"),
	}
}

type manager struct {
	cDocs   *mongo.Collection
	cDocOps *mongo.Collection
}

func rewriteMongoError(err error) error {
	if err == mongo.ErrNoDocuments {
		return &errors.ErrorDocNotFound{}
	}
	return err
}

func (m *manager) IsDocArchived(ctx context.Context, projectId primitive.ObjectID, docId primitive.ObjectID) (bool, error) {
	var doc models.DocInS3Field
	err := m.cDocs.FindOne(
		ctx,
		docFilter(projectId, docId),
		options.FindOne().SetProjection(getProjection(doc)),
	).Decode(&doc)
	if err != nil {
		return false, rewriteMongoError(err)
	}
	return doc.IsArchived(), nil
}

func (m *manager) IsDocDeleted(ctx context.Context, projectId primitive.ObjectID, docId primitive.ObjectID) (bool, error) {
	var doc models.DocDeletedField
	err := m.cDocs.FindOne(
		ctx,
		docFilter(projectId, docId),
		options.FindOne().SetProjection(getProjection(doc)),
	).Decode(&doc)
	if err != nil {
		return false, rewriteMongoError(err)
	}
	return doc.Deleted, nil
}

func (m *manager) CheckDocExists(ctx context.Context, projectId primitive.ObjectID, docId primitive.ObjectID) error {
	err := m.cDocs.FindOne(
		ctx,
		docFilter(projectId, docId),
		options.FindOne().SetProjection(docIdFieldProjection),
	).Err()
	return rewriteMongoError(err)
}

func (m *manager) GetDocContentsWithFullContext(ctx context.Context, projectId primitive.ObjectID, docId primitive.ObjectID) (*models.DocContentsWithFullContext, error) {
	var doc models.DocContentsWithFullContext
	err := m.cDocs.FindOne(
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

	var docVersion models.DocOpsVersionField
	err = m.cDocOps.FindOne(
		ctx,
		models.DocOpsDocIdField{DocId: docId},
		options.FindOne().SetProjection(getProjection(docVersion)),
	).Decode(&docVersion)
	if err != nil {
		return nil, rewriteMongoError(err)
	}
	doc.Version = docVersion.Version
	return &doc, nil
}

func (m *manager) GetDocForArchiving(ctx context.Context, projectId primitive.ObjectID, docId primitive.ObjectID) (*models.DocArchiveContext, error) {
	var doc models.DocArchiveContext
	err := m.cDocs.FindOne(
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

func (m *manager) GetDocLines(ctx context.Context, projectId primitive.ObjectID, docId primitive.ObjectID) (sharedTypes.Lines, error) {
	var doc models.DocLines
	err := m.cDocs.FindOne(
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

func (m *manager) PeakDeletedDocNames(ctx context.Context, projectId primitive.ObjectID, limit int64) ([]models.DocName, error) {
	docs := make(models.DocNameCollection, 0)
	res, err := m.cDocs.Find(
		ctx,
		projectFilterDeleted(projectId),
		options.Find().
			SetProjection(getProjection(docs)).
			SetSort(bson.M{
				"deletedAt": -1,
			}).
			SetLimit(limit),
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

func (m *manager) GetAllRanges(ctx context.Context, projectId primitive.ObjectID) ([]models.DocRanges, error) {
	docs := make(models.DocRangesCollection, 0)
	res, err := m.cDocs.Find(
		ctx,
		projectFilterNonDeleted(projectId),
		options.Find().SetProjection(getProjection(docs)),
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

func (m *manager) GetAllDocContents(ctx context.Context, projectId primitive.ObjectID) ([]models.DocContents, error) {
	docs := make(models.DocContentsCollection, 0)
	res, err := m.cDocs.Find(
		ctx,
		projectFilterNonDeleted(projectId),
		options.Find().SetProjection(getProjection(docs)),
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

func (m *manager) streamDocIds(ctx context.Context, filter bson.M, batchSize int32) (<-chan primitive.ObjectID, <-chan error) {
	ids := make(chan primitive.ObjectID)
	errs := make(chan error)

	go func() {
		defer close(ids)
		defer close(errs)

		cursor, err := m.cDocs.Find(
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
			var doc models.DocIdField
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

func (m *manager) GetDocIdsForDeletion(ctx context.Context, projectId primitive.ObjectID, batchSize int32) (<-chan primitive.ObjectID, <-chan error) {
	return m.streamDocIds(
		ctx,
		projectFilterAllDocs(projectId),
		batchSize,
	)
}

func (m *manager) GetDocIdsForArchiving(ctx context.Context, projectId primitive.ObjectID, batchSize int32) (<-chan primitive.ObjectID, <-chan error) {
	return m.streamDocIds(
		ctx,
		projectFilterNonArchivedDocs(projectId),
		batchSize,
	)
}

func (m *manager) GetDocIdsForUnArchiving(ctx context.Context, projectId primitive.ObjectID, batchSize int32) (<-chan primitive.ObjectID, <-chan error) {
	return m.streamDocIds(
		ctx,
		projectFilterNeedsUnArchiving(projectId),
		batchSize,
	)
}

type upsertDocUpdate struct {
	models.DocLinesField  `bson:"inline"`
	models.DocRangesField `bson:"inline"`
	ProjectId             primitive.ObjectID `bson:"project_id"`
}

func (m *manager) UpsertDoc(ctx context.Context, projectId primitive.ObjectID, docId primitive.ObjectID, lines sharedTypes.Lines, ranges sharedTypes.Ranges) (sharedTypes.Revision, error) {
	updates := upsertDocUpdate{ProjectId: projectId}
	updates.Lines = lines
	updates.Ranges = ranges

	var doc models.DocRevisionField
	err := m.cDocs.FindOneAndUpdate(
		ctx,
		models.DocIdField{Id: docId},
		bson.M{
			"$set":   updates,
			"$inc":   models.DocRevisionField{Revision: 1},
			"$unset": models.DocInS3Field{InS3: true},
		},
		options.FindOneAndUpdate().
			SetUpsert(true).
			SetProjection(getProjection(doc)).
			SetReturnDocument(options.After),
	).Decode(&doc)
	if err != nil {
		return 0, rewriteMongoError(err)
	}
	return doc.Revision, nil
}

func (m *manager) SetDocVersion(ctx context.Context, docId primitive.ObjectID, version sharedTypes.Version) error {
	_, err := m.cDocOps.UpdateOne(
		ctx,
		models.DocOpsDocIdField{DocId: docId},
		bson.M{
			"$set": models.DocOpsVersionField{Version: version},
		},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}

func (m *manager) PatchDocMeta(ctx context.Context, projectId primitive.ObjectID, docId primitive.ObjectID, meta models.DocMeta) error {
	_, err := m.cDocs.UpdateOne(
		ctx,
		docFilter(projectId, docId),
		bson.M{
			"$set": meta,
		},
	)
	return err
}

func (m *manager) MarkDocAsArchived(ctx context.Context, projectId primitive.ObjectID, docId primitive.ObjectID, revision sharedTypes.Revision) error {
	filter := docFilterWithRevision(projectId, docId, revision)
	_, err := m.cDocs.UpdateOne(
		ctx,
		filter,
		bson.M{
			"$set":   models.DocInS3Field{InS3: true},
			"$unset": docArchiveContentsFields,
		},
	)
	return rewriteMongoError(err)
}

func (m *manager) DestroyDoc(ctx context.Context, projectId primitive.ObjectID, docId primitive.ObjectID) error {
	_, err1 := m.cDocs.DeleteOne(
		ctx,
		docFilter(projectId, docId),
	)
	_, err2 := m.cDocOps.DeleteOne(
		ctx,
		models.DocOpsDocIdField{DocId: docId},
	)
	if err1 != nil {
		return rewriteMongoError(err1)
	}
	return rewriteMongoError(err2)
}
