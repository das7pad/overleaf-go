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

	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/document-updater/pkg/managers/documentUpdater/internal/docManager"
	"github.com/das7pad/document-updater/pkg/types"
)

type Manager interface {
	CheckDocExists(
		ctx context.Context,
		projectId primitive.ObjectID,
		docId primitive.ObjectID,
	) error
	GetDoc(
		ctx context.Context,
		projectId primitive.ObjectID,
		docId primitive.ObjectID,
		fromVersion types.Version,
	) (*types.GetDocResponse, error)
	GetProjectDocsAndFlushIfOld(ctx context.Context, projectId primitive.ObjectID, newState string) ([]*types.DocContent, error)
	FlushDoc(ctx context.Context, projectId, docId primitive.ObjectID) error
	FlushAndDeleteDoc(ctx context.Context, projectId, docId primitive.ObjectID) error
	FlushProject(ctx context.Context, projectId primitive.ObjectID) error
	FlushAndDeleteProject(ctx context.Context, projectId primitive.ObjectID) error
}

func New(options *types.Options, client redis.UniversalClient) (Manager, error) {
	dm, err := docManager.New(options, client)
	if err != nil {
		return nil, err
	}
	return &manager{
		dm: dm,
	}, nil
}

type manager struct {
	dm docManager.Manager
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

func (m *manager) FlushDoc(ctx context.Context, projectId, docId primitive.ObjectID) error {
	return m.dm.FlushDoc(ctx, projectId, docId)
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
