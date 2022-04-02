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

package docstore

import (
	"context"

	"github.com/edgedb/edgedb-go"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/doc"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/docstore/pkg/managers/docstore/internal/docArchive"
	"github.com/das7pad/overleaf-go/services/docstore/pkg/types"
)

type Modified bool

type Manager interface {
	IsDocDeleted(
		ctx context.Context,
		projectId edgedb.UUID,
		docId edgedb.UUID,
	) (bool, error)

	GetFullDoc(
		ctx context.Context,
		projectId edgedb.UUID,
		docId edgedb.UUID,
	) (*doc.ContentsWithFullContext, error)

	GetDocLines(
		ctx context.Context,
		projectId edgedb.UUID,
		docId edgedb.UUID,
	) (sharedTypes.Lines, error)

	PeakDeletedDocNames(
		ctx context.Context,
		projectId edgedb.UUID,
	) ([]doc.Name, error)

	GetAllRanges(
		ctx context.Context,
		projectId edgedb.UUID,
	) ([]doc.Ranges, error)

	GetAllDocContents(
		ctx context.Context,
		projectId edgedb.UUID,
	) (doc.ContentsCollection, error)

	CreateEmptyDoc(ctx context.Context, projectId, docId edgedb.UUID) error
	CreateDocWithContent(ctx context.Context, projectId, docId edgedb.UUID, snapshot sharedTypes.Snapshot) error
	CreateDocsWithContent(ctx context.Context, projectId edgedb.UUID, docs []doc.Contents) error

	UpdateDoc(
		ctx context.Context,
		projectId edgedb.UUID,
		docId edgedb.UUID,
		update *doc.ForDocUpdate,
	) (Modified, error)

	ArchiveProject(
		ctx context.Context,
		projectId edgedb.UUID,
	) error

	ArchiveDoc(
		ctx context.Context,
		projectId edgedb.UUID,
		docId edgedb.UUID,
	) error

	UnArchiveProject(
		ctx context.Context,
		projectId edgedb.UUID,
	) error

	DestroyProject(
		ctx context.Context,
		projectId edgedb.UUID,
	) error
}

func New(options *types.Options, c *edgedb.Client, db *mongo.Database) (Manager, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}

	dm := doc.New(c, db)

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
	maxDeletedDocs int64
}

func (m *manager) IsDocDeleted(ctx context.Context, projectId edgedb.UUID, docId edgedb.UUID) (bool, error) {
	return m.dm.IsDocDeleted(ctx, projectId, docId)
}

func (m *manager) recoverDocError(ctx context.Context, projectId edgedb.UUID, docId edgedb.UUID, err error) error {
	if errors.IsDocArchivedError(err) {
		return m.da.UnArchiveDoc(ctx, projectId, docId)
	}
	return err
}

func (m *manager) GetFullDoc(ctx context.Context, projectId edgedb.UUID, docId edgedb.UUID) (*doc.ContentsWithFullContext, error) {
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

func (m *manager) GetDocLines(ctx context.Context, projectId edgedb.UUID, docId edgedb.UUID) (sharedTypes.Lines, error) {
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

func (m *manager) PeakDeletedDocNames(ctx context.Context, projectId edgedb.UUID) ([]doc.Name, error) {
	return m.dm.PeakDeletedDocNames(ctx, projectId, m.maxDeletedDocs)
}

func (m *manager) GetAllRanges(ctx context.Context, projectId edgedb.UUID) ([]doc.Ranges, error) {
	for {
		ranges, err := m.dm.GetAllRanges(ctx, projectId)
		if err != nil {
			if errors.IsDocArchivedError(err) {
				if err = m.da.UnArchiveDocs(ctx, projectId); err != nil {
					return nil, err
				}
				// Retry after un-archiving.
				continue
			}
			return nil, err
		}
		return ranges, nil
	}
}

func (m *manager) GetAllDocContents(ctx context.Context, projectId edgedb.UUID) (doc.ContentsCollection, error) {
	for {
		contents, err := m.dm.GetAllDocContents(ctx, projectId)
		if err != nil {
			if errors.IsDocArchivedError(err) {
				if err = m.da.UnArchiveDocs(ctx, projectId); err != nil {
					return nil, err
				}
				// Retry after un-archiving.
				continue
			}
			return nil, err
		}
		return contents, nil
	}
}

func (m *manager) CreateEmptyDoc(ctx context.Context, projectId, docId edgedb.UUID) error {
	return m.dm.CreateDocWithContent(ctx, projectId, docId, nil)
}

func (m *manager) CreateDocWithContent(ctx context.Context, projectId, docId edgedb.UUID, snapshot sharedTypes.Snapshot) error {
	return m.dm.CreateDocWithContent(ctx, projectId, docId, snapshot)
}

func (m *manager) CreateDocsWithContent(ctx context.Context, projectId edgedb.UUID, docs []doc.Contents) error {
	return m.dm.CreateDocsWithContent(ctx, projectId, docs)
}

func (m *manager) UpdateDoc(ctx context.Context, projectId edgedb.UUID, docId edgedb.UUID, update *doc.ForDocUpdate) (Modified, error) {
	if err := update.Snapshot.Validate(); err != nil {
		return false, err
	}
	if err := m.dm.UpdateDoc(ctx, projectId, docId, update); err != nil {
		return false, err
	}
	return true, nil
}

func (m *manager) ArchiveProject(ctx context.Context, projectId edgedb.UUID) error {
	return m.da.ArchiveDocs(ctx, projectId)
}

func (m *manager) ArchiveDoc(ctx context.Context, projectId edgedb.UUID, docId edgedb.UUID) error {
	return m.da.ArchiveDoc(ctx, projectId, docId)
}

func (m *manager) UnArchiveProject(ctx context.Context, projectId edgedb.UUID) error {
	return m.da.UnArchiveDocs(ctx, projectId)
}

func (m *manager) DestroyProject(ctx context.Context, projectId edgedb.UUID) error {
	return m.da.DestroyDocs(ctx, projectId)
}
