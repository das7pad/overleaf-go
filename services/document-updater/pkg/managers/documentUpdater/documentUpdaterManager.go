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

package documentUpdater

import (
	"context"

	"github.com/edgedb/edgedb-go"
	"github.com/go-redis/redis/v8"

	"github.com/das7pad/overleaf-go/pkg/edgedbTx"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater/internal/dispatchManager"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater/internal/docManager"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
)

type Manager interface {
	StartBackgroundTasks(ctx context.Context)
	CheckDocExists(
		ctx context.Context,
		projectId edgedb.UUID,
		docId edgedb.UUID,
	) error
	GetDoc(
		ctx context.Context,
		projectId edgedb.UUID,
		docId edgedb.UUID,
		fromVersion sharedTypes.Version,
	) (*types.GetDocResponse, error)
	GetProjectDocsAndFlushIfOldLines(ctx context.Context, projectId edgedb.UUID) ([]*types.DocContentLines, error)
	GetProjectDocsAndFlushIfOldSnapshot(ctx context.Context, projectId edgedb.UUID) (types.DocContentSnapshots, error)
	FlushDocIfLoaded(ctx context.Context, projectId, docId edgedb.UUID) error
	FlushAndDeleteDoc(ctx context.Context, projectId, docId edgedb.UUID) error
	FlushProject(ctx context.Context, projectId edgedb.UUID) error
	FlushAndDeleteProject(ctx context.Context, projectId edgedb.UUID) error
	SetDoc(ctx context.Context, projectId, docId edgedb.UUID, request *types.SetDocRequest) error
	ProcessProjectUpdates(ctx context.Context, projectId edgedb.UUID, request *types.ProcessProjectUpdatesRequest) error
}

func New(options *types.Options, c *edgedb.Client, client redis.UniversalClient) (Manager, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}

	dm, err := docManager.New(options, c, client)
	if err != nil {
		return nil, err
	}
	return &manager{
		dispatcher: dispatchManager.New(options, client, dm),
		dm:         dm,
	}, nil
}

type manager struct {
	dispatcher dispatchManager.Manager
	dm         docManager.Manager
}

func (m *manager) StartBackgroundTasks(ctx context.Context) {
	m.dispatcher.Start(ctx)
}

func (m *manager) ProcessProjectUpdates(ctx context.Context, projectId edgedb.UUID, request *types.ProcessProjectUpdatesRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}

	subVersion := sharedTypes.Version(0)
	base := request.ProjectVersion.String()
	for _, update := range request.Updates {
		subVersion += 1
		update.Version = base + "." + subVersion.String()
		switch update.Type {
		case "rename-doc":
			err := m.dm.RenameDoc(ctx, projectId, update.RenameDocUpdate())
			if err != nil {
				return err
			}
		case "rename-file":
			// noop
		case "add-doc":
			// noop
		case "add-file":
			// noop
		}
	}
	return nil
}

func (m *manager) CheckDocExists(ctx context.Context, projectId, docId edgedb.UUID) error {
	_, err := m.dm.GetDoc(ctx, projectId, docId)
	return err
}

func (m *manager) GetDoc(ctx context.Context, projectId, docId edgedb.UUID, fromVersion sharedTypes.Version) (*types.GetDocResponse, error) {
	response := &types.GetDocResponse{}
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
	return response, nil
}

func (m *manager) GetProjectDocsAndFlushIfOldLines(ctx context.Context, projectId edgedb.UUID) ([]*types.DocContentLines, error) {
	if err := edgedbTx.CheckNotInTx(ctx); err != nil {
		return nil, err
	}
	docs, err := m.dm.GetProjectDocsAndFlushIfOld(ctx, projectId)
	if err != nil {
		return nil, err
	}
	docContentsLines := make([]*types.DocContentLines, len(docs))
	for i, doc := range docs {
		docContentsLines[i] = doc.ToDocContentLines()
	}
	return docContentsLines, nil
}

func (m *manager) GetProjectDocsAndFlushIfOldSnapshot(ctx context.Context, projectId edgedb.UUID) (types.DocContentSnapshots, error) {
	if err := edgedbTx.CheckNotInTx(ctx); err != nil {
		return nil, err
	}
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

func (m *manager) SetDoc(ctx context.Context, projectId, docId edgedb.UUID, request *types.SetDocRequest) error {
	if err := edgedbTx.CheckNotInTx(ctx); err != nil {
		return err
	}
	return m.dm.SetDoc(ctx, projectId, docId, request)
}

func (m *manager) FlushDocIfLoaded(ctx context.Context, projectId, docId edgedb.UUID) error {
	if err := edgedbTx.CheckNotInTx(ctx); err != nil {
		return err
	}
	return m.dm.FlushDocIfLoaded(ctx, projectId, docId)
}

func (m *manager) FlushAndDeleteDoc(ctx context.Context, projectId, docId edgedb.UUID) error {
	if err := edgedbTx.CheckNotInTx(ctx); err != nil {
		return err
	}
	return m.dm.FlushAndDeleteDoc(ctx, projectId, docId)
}

func (m *manager) FlushProject(ctx context.Context, projectId edgedb.UUID) error {
	if err := edgedbTx.CheckNotInTx(ctx); err != nil {
		return err
	}
	return m.dm.FlushProject(ctx, projectId)
}

func (m *manager) FlushAndDeleteProject(ctx context.Context, projectId edgedb.UUID) error {
	if err := edgedbTx.CheckNotInTx(ctx); err != nil {
		return err
	}
	return m.dm.FlushAndDeleteProject(ctx, projectId)
}
