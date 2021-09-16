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

package docstore

import (
	"context"
	"math"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/doc"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/docstore/pkg/managers/docstore/internal/docArchive"
	"github.com/das7pad/overleaf-go/services/docstore/pkg/types"
)

type Modified bool

const DefaultLimit types.Limit = -1

type Manager interface {
	IsDocDeleted(
		ctx context.Context,
		projectId primitive.ObjectID,
		docId primitive.ObjectID,
	) (bool, error)

	GetFullDoc(
		ctx context.Context,
		projectId primitive.ObjectID,
		docId primitive.ObjectID,
	) (*doc.ContentsWithFullContext, error)

	GetDocLines(
		ctx context.Context,
		projectId primitive.ObjectID,
		docId primitive.ObjectID,
	) (sharedTypes.Lines, error)

	PeakDeletedDocNames(
		ctx context.Context,
		projectId primitive.ObjectID,
		limit types.Limit,
	) ([]doc.Name, error)

	GetAllRanges(
		ctx context.Context,
		projectId primitive.ObjectID,
	) ([]doc.Ranges, error)

	GetAllDocContents(
		ctx context.Context,
		projectId primitive.ObjectID,
	) ([]doc.Contents, error)

	UpdateDoc(
		ctx context.Context,
		projectId primitive.ObjectID,
		docId primitive.ObjectID,
		lines sharedTypes.Lines,
		version sharedTypes.Version,
		ranges sharedTypes.Ranges,
	) (Modified, sharedTypes.Revision, error)

	PatchDoc(
		ctx context.Context,
		projectId primitive.ObjectID,
		docId primitive.ObjectID,
		meta doc.Meta,
	) error

	ArchiveProject(
		ctx context.Context,
		projectId primitive.ObjectID,
	) error

	ArchiveDoc(
		ctx context.Context,
		projectId primitive.ObjectID,
		docId primitive.ObjectID,
	) error

	UnArchiveProject(
		ctx context.Context,
		projectId primitive.ObjectID,
	) error

	DestroyProject(
		ctx context.Context,
		projectId primitive.ObjectID,
	) error
}

func New(options *types.Options, db *mongo.Database) (Manager, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}

	dm := doc.New(db)

	da, err := docArchive.New(options, dm)
	if err != nil {
		return nil, err
	}
	return &manager{
		da:             da,
		dm:             dm,
		maxDeletedDocs: options.MaxDeletedDocs,
	}, nil
}

type manager struct {
	da             docArchive.Manager
	dm             doc.Manager
	maxDeletedDocs types.Limit
}

func (m *manager) IsDocDeleted(ctx context.Context, projectId primitive.ObjectID, docId primitive.ObjectID) (bool, error) {
	return m.dm.IsDocDeleted(ctx, projectId, docId)
}

func (m *manager) recoverDocError(ctx context.Context, projectId primitive.ObjectID, docId primitive.ObjectID, err error) error {
	if errors.IsDocArchivedError(err) {
		return m.da.UnArchiveDoc(ctx, projectId, docId)
	}
	return err
}

func (m *manager) GetFullDoc(ctx context.Context, projectId primitive.ObjectID, docId primitive.ObjectID) (*doc.ContentsWithFullContext, error) {
	for {
		d, err := m.dm.GetDocContentsWithFullContext(ctx, projectId, docId)
		if err != nil {
			if err = m.recoverDocError(ctx, projectId, docId, err); err != nil {
				return nil, err
			}
			// The doc has been un-archived, retry.
			continue
		}
		return d, nil
	}
}

func (m *manager) GetDocLines(ctx context.Context, projectId primitive.ObjectID, docId primitive.ObjectID) (sharedTypes.Lines, error) {
	for {
		lines, err := m.dm.GetDocLines(ctx, projectId, docId)
		if err != nil {
			if err = m.recoverDocError(ctx, projectId, docId, err); err != nil {
				return nil, err
			}
			// The doc has been un-archived, retry.
			continue
		}
		return lines, nil
	}
}

func (m *manager) PeakDeletedDocNames(ctx context.Context, projectId primitive.ObjectID, limit types.Limit) ([]doc.Name, error) {
	if limit == DefaultLimit {
		limit = m.maxDeletedDocs
	} else if limit < 1 {
		return nil, &errors.ValidationError{
			Msg: "limit must be greater or equal 1",
		}
	} else {
		// Silently limit the provided value to the configured default limit.
		limit = types.Limit(math.Min(
			float64(limit),
			float64(m.maxDeletedDocs),
		))
	}

	return m.dm.PeakDeletedDocNames(ctx, projectId, int64(limit))
}

func (m *manager) GetAllRanges(ctx context.Context, projectId primitive.ObjectID) ([]doc.Ranges, error) {
	for {
		if err := m.da.UnArchiveDocs(ctx, projectId); err != nil {
			return nil, err
		}
		ranges, err := m.dm.GetAllRanges(ctx, projectId)
		if err != nil {
			if errors.IsDocArchivedError(err) {
				// Retry after un-archiving.
				continue
			}
			return nil, err
		}
		return ranges, nil
	}
}

func (m *manager) GetAllDocContents(ctx context.Context, projectId primitive.ObjectID) ([]doc.Contents, error) {
	for {
		if err := m.da.UnArchiveDocs(ctx, projectId); err != nil {
			return nil, err
		}
		contents, err := m.dm.GetAllDocContents(ctx, projectId)
		if err != nil {
			if errors.IsDocArchivedError(err) {
				// Retry after un-archiving.
				continue
			}
			return nil, err
		}
		return contents, nil
	}
}

const MaxLineLength = 2 * 1024 * 1024

func validateDocLines(lines sharedTypes.Lines) error {
	if lines == nil {
		return &errors.ValidationError{Msg: "no doc lines provided"}
	}
	sum := 0
	for _, line := range lines {
		sum += len(line)
	}
	if sum > MaxLineLength {
		return &errors.BodyTooLargeError{}
	}
	return nil
}

func (m *manager) UpdateDoc(ctx context.Context, projectId primitive.ObjectID, docId primitive.ObjectID, lines sharedTypes.Lines, version sharedTypes.Version, ranges sharedTypes.Ranges) (Modified, sharedTypes.Revision, error) {
	if err := validateDocLines(lines); err != nil {
		return false, 0, err
	}

	var modifiedContents bool
	var modifiedVersion bool
	var revision sharedTypes.Revision

	if d, err := m.GetFullDoc(ctx, projectId, docId); err == nil {
		modifiedContents = false
		modifiedVersion = false
		if !d.Lines.Equals(lines) {
			modifiedContents = true
		}
		if !d.Ranges.Equals(ranges) {
			modifiedContents = true
		}
		if !d.Version.Equals(version) {
			modifiedVersion = true
		}
		revision = d.Revision
	} else if errors.IsDocNotFoundError(err) {
		if version != 0 {
			// Block 'creation' of documents with non-zero version.
			return false, 0, err
		}

		modifiedContents = true
		modifiedVersion = true
		revision = 0
	} else {
		return false, 0, err
	}

	if !modifiedContents && !modifiedVersion {
		return false, revision, nil
	}
	if modifiedContents {
		rev, err := m.dm.UpsertDoc(
			ctx,
			projectId,
			docId,
			lines,
			ranges,
		)
		if err != nil {
			return false, 0, err
		}
		revision = rev
	}
	if modifiedVersion {
		err := m.dm.SetDocVersion(
			ctx,
			docId,
			version,
		)
		if err != nil {
			return false, 0, err
		}
	}
	return true, revision, nil
}

func (m *manager) PatchDoc(ctx context.Context, projectId primitive.ObjectID, docId primitive.ObjectID, meta doc.Meta) error {
	if meta.Deleted {
		if meta.Name == "" {
			return &errors.ValidationError{Msg: "missing name when deleting"}
		}
		if meta.DeletedAt.IsZero() {
			return &errors.ValidationError{
				Msg: "missing deletedAt when deleting",
			}
		}
	}
	if err := m.dm.CheckDocExists(ctx, projectId, docId); err != nil {
		return err
	}
	if meta.Deleted {
		if err := m.da.ArchiveDoc(ctx, projectId, docId); err != nil {
			return err
		}
	}
	return m.dm.PatchDocMeta(ctx, projectId, docId, meta)
}

func (m *manager) ArchiveProject(ctx context.Context, projectId primitive.ObjectID) error {
	return m.da.ArchiveDocs(ctx, projectId)
}

func (m *manager) ArchiveDoc(ctx context.Context, projectId primitive.ObjectID, docId primitive.ObjectID) error {
	return m.da.ArchiveDoc(ctx, projectId, docId)
}

func (m *manager) UnArchiveProject(ctx context.Context, projectId primitive.ObjectID) error {
	return m.da.UnArchiveDocs(ctx, projectId)
}

func (m *manager) DestroyProject(ctx context.Context, projectId primitive.ObjectID) error {
	return m.da.DestroyDocs(ctx, projectId)
}
