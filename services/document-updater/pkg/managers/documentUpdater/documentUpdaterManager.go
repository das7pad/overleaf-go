// Golang port of Overleaf
// Copyright (C) 2021-2023 Jakob Ackermann <das7pad@outlook.com>
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

package documentUpdater

import (
	"context"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater/internal/dispatchManager"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater/internal/docManager"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater/internal/realTimeRedisManager"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
)

type Manager interface {
	dispatchManager.Manager
	CheckDocExists(ctx context.Context, projectId sharedTypes.UUID, docId sharedTypes.UUID) error
	GetDoc(ctx context.Context, projectId sharedTypes.UUID, docId sharedTypes.UUID, fromVersion sharedTypes.Version) (*types.GetDocResponse, error)
	GetProjectDocsAndFlushIfOldSnapshot(ctx context.Context, projectId sharedTypes.UUID) (types.DocContentSnapshots, error)
	FlushAndDeleteDoc(ctx context.Context, projectId, docId sharedTypes.UUID) error
	FlushProject(ctx context.Context, projectId sharedTypes.UUID) error
	FlushAndDeleteProject(ctx context.Context, projectId sharedTypes.UUID) error
	SetDoc(ctx context.Context, projectId, docId sharedTypes.UUID, request types.SetDocRequest) error
	ProcessProjectUpdates(ctx context.Context, projectId sharedTypes.UUID, updates types.RenameDocUpdates) error
}

func New(options *types.Options, db *pgxpool.Pool, client redis.UniversalClient) (Manager, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	rtRm, err := realTimeRedisManager.New(client)
	if err != nil {
		return nil, err
	}
	dm, err := docManager.New(db, client, rtRm)
	if err != nil {
		return nil, err
	}
	return &manager{
		dispatcher: dispatchManager.New(options, client, dm, rtRm),
		dm:         dm,
	}, nil
}

type dispatcher dispatchManager.Manager

type manager struct {
	dispatcher
	dm docManager.Manager
}

func (m *manager) ProcessProjectUpdates(ctx context.Context, projectId sharedTypes.UUID, updates types.RenameDocUpdates) error {
	if err := updates.Validate(); err != nil {
		return err
	}

	for _, update := range updates {
		err := m.dm.RenameDoc(ctx, projectId, update.DocId, update.NewPath)
		if err != nil {
			return errors.Tag(err, update.DocId.String())
		}
	}
	return nil
}

func (m *manager) CheckDocExists(ctx context.Context, projectId, docId sharedTypes.UUID) error {
	_, err := m.dm.GetDoc(ctx, projectId, docId)
	return err
}

func (m *manager) GetDoc(ctx context.Context, projectId, docId sharedTypes.UUID, fromVersion sharedTypes.Version) (*types.GetDocResponse, error) {
	response := types.GetDocResponse{}
	if fromVersion == -1 {
		doc, err := m.dm.GetDoc(ctx, projectId, docId)
		if err != nil {
			return nil, err
		}
		response.Ops = make([]sharedTypes.DocumentUpdate, 0)
		response.PathName = doc.PathName
		response.Snapshot = string(doc.Snapshot)
		response.Version = doc.Version
	} else {
		doc, updates, err := m.dm.GetDocAndRecentUpdates(
			ctx, projectId, docId, fromVersion,
		)
		if err != nil {
			return nil, err
		}
		response.Ops = updates
		response.Version = doc.Version
	}
	return &response, nil
}

func (m *manager) GetProjectDocsAndFlushIfOldSnapshot(ctx context.Context, projectId sharedTypes.UUID) (types.DocContentSnapshots, error) {
	docs, err := m.dm.GetProjectDocsAndFlushIfOld(ctx, projectId)
	if err != nil {
		return nil, err
	}
	docContentsSnapshot := make(types.DocContentSnapshots, len(docs))
	for i, doc := range docs {
		docContentsSnapshot[i] = doc.ToDocContentSnapshot()
	}
	return docContentsSnapshot, nil
}

func (m *manager) SetDoc(ctx context.Context, projectId, docId sharedTypes.UUID, request types.SetDocRequest) error {
	return m.dm.SetDoc(ctx, projectId, docId, request)
}

func (m *manager) FlushAndDeleteDoc(ctx context.Context, projectId, docId sharedTypes.UUID) error {
	return m.dm.FlushAndDeleteDoc(ctx, projectId, docId)
}

func (m *manager) FlushProject(ctx context.Context, projectId sharedTypes.UUID) error {
	return m.dm.FlushProject(ctx, projectId)
}

func (m *manager) FlushAndDeleteProject(ctx context.Context, projectId sharedTypes.UUID) error {
	return m.dm.FlushAndDeleteProject(ctx, projectId)
}
