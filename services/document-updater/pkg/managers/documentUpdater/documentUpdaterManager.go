// Golang port of the Overleaf document-updater service
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
	"strconv"

	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/document-updater/pkg/managers/documentUpdater/internal/dispatchManager"
	"github.com/das7pad/document-updater/pkg/managers/documentUpdater/internal/docManager"
	"github.com/das7pad/document-updater/pkg/types"
)

type Manager interface {
	StartBackgroundTasks(ctx context.Context)
	CheckDocExists(
		ctx context.Context,
		projectId primitive.ObjectID,
		docId primitive.ObjectID,
	) error
	ClearProjectState(ctx context.Context, projectId primitive.ObjectID) error
	GetDoc(
		ctx context.Context,
		projectId primitive.ObjectID,
		docId primitive.ObjectID,
		fromVersion types.Version,
	) (*types.GetDocResponse, error)
	GetProjectDocsAndFlushIfOld(ctx context.Context, projectId primitive.ObjectID, newState string) ([]*types.DocContent, error)
	FlushDocIfLoaded(ctx context.Context, projectId, docId primitive.ObjectID) error
	FlushAndDeleteDoc(ctx context.Context, projectId, docId primitive.ObjectID) error
	FlushProject(ctx context.Context, projectId primitive.ObjectID) error
	FlushAndDeleteProject(ctx context.Context, projectId primitive.ObjectID) error
	RenameDoc(ctx context.Context, projectId primitive.ObjectID, update *types.RenameUpdate) error
	SetDoc(ctx context.Context, projectId, docId primitive.ObjectID, request *types.SetDocRequest) error
	ProcessProjectUpdates(ctx context.Context, projectId primitive.ObjectID, request *types.ProcessProjectUpdatesRequest) error
}

func New(options *types.Options, client redis.UniversalClient) (Manager, error) {
	dm, err := docManager.New(options, client)
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

func (m *manager) RenameDoc(ctx context.Context, projectId primitive.ObjectID, update *types.RenameUpdate) error {
	if err := update.Validate(); err != nil {
		return err
	}
	return m.dm.RenameDoc(ctx, projectId, update)
}

func (m *manager) ProcessProjectUpdates(ctx context.Context, projectId primitive.ObjectID, request *types.ProcessProjectUpdatesRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}

	subVersion := int64(0)
	base := strconv.FormatInt(int64(request.ProjectVersion), 10)
	for _, update := range request.Updates {
		subVersion += 1
		update.Version = base + "." + strconv.FormatInt(subVersion, 10)
		switch update.Type {
		case "rename-doc":
			err := m.dm.RenameDoc(ctx, projectId, update.RenameUpdate())
			if err != nil {
				return err
			}
		case "rename-file":
		case "add-doc":
		case "add-file":
		}
	}
	return nil
}

func (m *manager) ClearProjectState(ctx context.Context, projectId primitive.ObjectID) error {
	return m.dm.ClearProjectState(ctx, projectId)
}

func (m *manager) CheckDocExists(ctx context.Context, projectId, docId primitive.ObjectID) error {
	_, err := m.dm.GetDoc(ctx, projectId, docId)
	return err
}

func (m *manager) GetDoc(ctx context.Context, projectId, docId primitive.ObjectID, fromVersion types.Version) (*types.GetDocResponse, error) {
	response := &types.GetDocResponse{}
	if fromVersion == -1 {
		doc, err := m.dm.GetDoc(ctx, projectId, docId)
		if err != nil {
			return nil, err
		}
		response.Ops = make([]types.DocumentUpdate, 0)
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

func (m *manager) GetProjectDocsAndFlushIfOld(ctx context.Context, projectId primitive.ObjectID, newState string) ([]*types.DocContent, error) {
	return m.dm.GetProjectDocsAndFlushIfOld(ctx, projectId, newState)
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
