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

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/docOps"
	"github.com/das7pad/overleaf-go/pkg/mongoTx"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
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
	) (*ContentsWithFullContext, error)

	GetDocForArchiving(
		ctx context.Context,
		projectId primitive.ObjectID,
		docId primitive.ObjectID,
	) (*ArchiveContext, error)

	GetDocLines(
		ctx context.Context,
		projectId primitive.ObjectID,
		docId primitive.ObjectID,
	) (sharedTypes.Lines, error)

	PeakDeletedDocNames(
		ctx context.Context,
		projectId primitive.ObjectID,
		limit int64,
	) ([]Name, error)

	GetAllRanges(
		ctx context.Context,
		projectId primitive.ObjectID,
	) ([]Ranges, error)

	GetAllDocContents(
		ctx context.Context,
		projectId primitive.ObjectID,
	) (ContentsCollection, error)

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

	CreateDocWithContent(ctx context.Context, projectId, docId primitive.ObjectID, snapshot sharedTypes.Snapshot) error
	CreateDocsWithContent(ctx context.Context, projectId primitive.ObjectID, docs []Contents) error

	UpsertDoc(
		ctx context.Context,
		projectId primitive.ObjectID,
		docId primitive.ObjectID,
		lines sharedTypes.Lines,
		ranges sharedTypes.Ranges,
	) error

	SetDocVersion(
		ctx context.Context,
		docId primitive.ObjectID,
		version sharedTypes.Version,
	) error

	PatchDocMeta(
		ctx context.Context,
		projectId primitive.ObjectID,
		docId primitive.ObjectID,
		docMeta Meta,
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

const (
	prefetchN                = 100
	ExternalVersionTombstone = -42
)

type manager struct {
	db      *mongo.Database
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
	var doc InS3Field
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
	var doc DeletedField
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

func (m *manager) GetDocContentsWithFullContext(ctx context.Context, projectId primitive.ObjectID, docId primitive.ObjectID) (*ContentsWithFullContext, error) {
	var doc ContentsWithFullContext
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
	if doc.Version != ExternalVersionTombstone {
		return &doc, nil
	}

	var docVersion docOps.VersionField
	err = m.cDocOps.FindOne(
		ctx,
		docOps.DocIdField{DocId: docId},
		options.FindOne().SetProjection(getProjection(docVersion)),
	).Decode(&docVersion)
	if err != nil {
		return nil, rewriteMongoError(err)
	}
	doc.Version = docVersion.Version
	return &doc, nil
}

func (m *manager) GetDocForArchiving(ctx context.Context, projectId primitive.ObjectID, docId primitive.ObjectID) (*ArchiveContext, error) {
	var doc ArchiveContext
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
	var doc Lines
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

func (m *manager) PeakDeletedDocNames(ctx context.Context, projectId primitive.ObjectID, limit int64) ([]Name, error) {
	docs := make(NameCollection, 0)
	res, err := m.cDocs.Find(
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

func (m *manager) GetAllRanges(ctx context.Context, projectId primitive.ObjectID) ([]Ranges, error) {
	docs := make(RangesCollection, 0)
	res, err := m.cDocs.Find(
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

func (m *manager) GetAllDocContents(ctx context.Context, projectId primitive.ObjectID) (ContentsCollection, error) {
	docs := make(ContentsCollection, 0)
	res, err := m.cDocs.Find(
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

func (m *manager) CreateDocWithContent(ctx context.Context, projectId, docId primitive.ObjectID, snapshot sharedTypes.Snapshot) error {
	return mongoTx.For(m.db, ctx, func(sCtx context.Context) error {
		eg, pCtx := errgroup.WithContext(sCtx)
		eg.Go(func() error {
			err := m.UpsertDoc(pCtx, projectId, docId, snapshot.ToLines(), sharedTypes.Ranges{})
			if err != nil {
				return errors.Tag(err, "cannot set doc")
			}
			return nil
		})
		eg.Go(func() error {
			if err := m.SetDocVersion(pCtx, docId, 0); err != nil {
				return errors.Tag(err, "cannot set doc version")
			}
			return nil
		})
		return eg.Wait()
	})
}

func (m *manager) CreateDocsWithContent(ctx context.Context, projectId primitive.ObjectID, docs []Contents) error {
	return mongoTx.For(m.db, ctx, func(sCtx context.Context) error {
		eg, pCtx := errgroup.WithContext(sCtx)
		eg.Go(func() error {
			docContents := make([]interface{}, len(docs))
			for i, doc := range docs {
				docContents[i] = forInsertion{
					IdField:        doc.IdField,
					LinesField:     doc.LinesField,
					ProjectIdField: ProjectIdField{ProjectId: projectId},
					VersionField: VersionField{
						Version: ExternalVersionTombstone,
					},
				}
			}
			if _, err := m.cDocs.InsertMany(pCtx, docContents); err != nil {
				return errors.Tag(err, "cannot insert doc contents")
			}
			return nil
		})
		eg.Go(func() error {
			docVersions := make([]interface{}, len(docs))
			for i, doc := range docs {
				docVersions[i] = docOps.Full{
					DocIdField: docOps.DocIdField{DocId: doc.Id},
				}
			}
			if _, err := m.cDocOps.InsertMany(pCtx, docVersions); err != nil {
				return errors.Tag(err, "cannot insert doc versions")
			}
			return nil
		})
		return eg.Wait()
	})
}

type upsertDocUpdate struct {
	LinesField   `bson:"inline"`
	RangesField  `bson:"inline"`
	VersionField `bson:"inline"`
}

func (m *manager) UpsertDoc(ctx context.Context, projectId primitive.ObjectID, docId primitive.ObjectID, lines sharedTypes.Lines, ranges sharedTypes.Ranges) error {
	updates := upsertDocUpdate{}
	updates.Lines = lines
	updates.Ranges = ranges
	updates.Version = ExternalVersionTombstone

	_, err := m.cDocs.UpdateOne(
		ctx,
		docFilter(projectId, docId),
		bson.M{
			"$set":   updates,
			"$inc":   RevisionField{Revision: 1},
			"$unset": InS3Field{InS3: true},
		},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}

func (m *manager) SetDocVersion(ctx context.Context, docId primitive.ObjectID, version sharedTypes.Version) error {
	_, err := m.cDocOps.UpdateOne(
		ctx,
		docOps.DocIdField{DocId: docId},
		bson.M{
			"$set": docOps.VersionField{Version: version},
		},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}

func (m *manager) PatchDocMeta(ctx context.Context, projectId primitive.ObjectID, docId primitive.ObjectID, meta Meta) error {
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
			"$set":   InS3Field{InS3: true},
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
		docOps.DocIdField{DocId: docId},
	)
	if err1 != nil {
		return rewriteMongoError(err1)
	}
	return rewriteMongoError(err2)
}
