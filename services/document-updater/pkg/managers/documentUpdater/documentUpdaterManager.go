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

	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater/internal/dispatchManager"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater/internal/docManager"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
)

type Manager interface {
	StartBackgroundTasks(ctx context.Context)
	CheckDocExists(
		ctx context.Context,
		projectId primitive.ObjectID,
		docId primitive.ObjectID,
	) error
	GetDoc(
		ctx context.Context,
		projectId primitive.ObjectID,
		docId primitive.ObjectID,
		fromVersion sharedTypes.Version,
	) (*types.GetDocResponse, error)
	GetProjectDocsAndFlushIfOldLines(ctx context.Context, projectId primitive.ObjectID) ([]*types.DocContentLines, error)
	GetProjectDocsAndFlushIfOldSnapshot(ctx context.Context, projectId primitive.ObjectID) (types.DocContentSnapshots, error)
	FlushDocIfLoaded(ctx context.Context, projectId, docId primitive.ObjectID) error
	FlushAndDeleteDoc(ctx context.Context, projectId, docId primitive.ObjectID) error
	FlushProject(ctx context.Context, projectId primitive.ObjectID) error
	FlushAndDeleteProject(ctx context.Context, projectId primitive.ObjectID) error
	SetDoc(ctx context.Context, projectId, docId primitive.ObjectID, request *types.SetDocRequest) error
	ProcessProjectUpdates(ctx context.Context, projectId primitive.ObjectID, request *types.ProcessProjectUpdatesRequest) error
}

func New(options *types.Options, client redis.UniversalClient, db *mongo.Database) (Manager, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}

	dm, err := docManager.New(options, client, db)
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

func (m *manager) ProcessProjectUpdates(ctx context.Context, projectId primitive.ObjectID, request *types.ProcessProjectUpdatesRequest) error {
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

func (m *manager) CheckDocExists(ctx context.Context, projectId, docId primitive.ObjectID) error {
	_, err := m.dm.GetDoc(ctx, projectId, docId)
	return err
}

func (m *manager) GetDoc(ctx context.Context, projectId, docId primitive.ObjectID, fromVersion sharedTypes.Version) (*types.GetDocResponse, error) {
	response := &types.GetDocResponse{}
	if fromVersion == -1 {
		doc, err := m.dm.GetDoc(ctx, projectId, docId)
		if err != nil {
			return nil, err
		}
		response.Ops = make([]sharedTypes.DocumentUpdate, 0)
		response.PathName = doc.PathName
		response.Snapshot = doc.Snapshot
		response.Ranges = doc.Ranges
		response.Version = doc.Version
	} else {
		doc, updates, err := m.dm.GetDocAndRecentUpdates(
			ctx, projectId, docId, fromVersion,
		)
		if err != nil {
			return nil, err
		}
		response.Ops = updates
		response.Ranges = doc.Ranges
		response.Version = doc.Version
	}
	return response, nil
}

func (m *manager) GetProjectDocsAndFlushIfOldLines(ctx context.Context, projectId primitive.ObjectID) ([]*types.DocContentLines, error) {
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

func (m *manager) GetProjectDocsAndFlushIfOldSnapshot(ctx context.Context, projectId primitive.ObjectID) (types.DocContentSnapshots, error) {
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

func (m *manager) SetDoc(ctx context.Context, projectId, docId primitive.ObjectID, request *types.SetDocRequest) error {
	return m.dm.SetDoc(ctx, projectId, docId, request)
}

func (m *manager) FlushDocIfLoaded(ctx context.Context, projectId, docId primitive.ObjectID) error {
	return m.dm.FlushDocIfLoaded(ctx, projectId, docId)
}

func (m *manager) FlushAndDeleteDoc(ctx context.Context, projectId, docId primitive.ObjectID) error {
	return m.dm.FlushAndDeleteDoc(ctx, projectId, docId)
}

func (m *manager) FlushProject(ctx context.Context, projectId primitive.ObjectID) error {
	return m.dm.FlushProject(ctx, projectId)
}

func (m *manager) FlushAndDeleteProject(ctx context.Context, projectId primitive.ObjectID) error {
	return m.dm.FlushAndDeleteProject(ctx, projectId)
}
