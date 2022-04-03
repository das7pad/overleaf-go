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

package projectMetadata

import (
	"context"
	"encoding/json"
	"time"

	"github.com/edgedb/edgedb-go"
	"github.com/go-redis/redis/v8"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/pubSub/channel"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/docstore/pkg/managers/docstore"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater"
	documentUpdaterTypes "github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	BroadcastMetadataForDoc(projectId, docId edgedb.UUID) error
	GetMetadataForProject(ctx context.Context, projectId edgedb.UUID) (*types.ProjectMetadataResponse, error)
	GetMetadataForDoc(ctx context.Context, projectId, docId edgedb.UUID, request *types.ProjectDocMetadataRequest) (*types.ProjectDocMetadataResponse, error)
}

func New(client redis.UniversalClient, editorEvents channel.Writer, pm project.Manager, dm docstore.Manager, dum documentUpdater.Manager) Manager {
	return &manager{
		client: client,
		c:      editorEvents,
		pm:     pm,
		dm:     dm,
		dum:    dum,
	}
}

type manager struct {
	client redis.UniversalClient
	c      channel.Writer
	pm     project.Manager
	dm     docstore.Manager
	dum    documentUpdater.Manager
}

func (m *manager) GetMetadataForProject(ctx context.Context, projectId edgedb.UUID) (*types.ProjectMetadataResponse, error) {
	l, err := m.getForProjectWithCache(ctx, projectId)
	if err != nil {
		return nil, err
	}
	p := make(types.ProjectMetadata, len(l))
	for id, metadata := range l {
		p[id] = inflate(metadata)
	}
	return &types.ProjectMetadataResponse{ProjectMetadata: p}, nil
}

func (m *manager) BroadcastMetadataForDoc(projectId, docId edgedb.UUID) error {
	r := &types.ProjectDocMetadataRequest{Broadcast: true}
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	_, err := m.GetMetadataForDoc(ctx, projectId, docId, r)
	return err
}

func (m *manager) GetMetadataForDoc(ctx context.Context, projectId, docId edgedb.UUID, request *types.ProjectDocMetadataRequest) (*types.ProjectDocMetadataResponse, error) {
	d, err := m.dum.GetDoc(ctx, projectId, docId, -1)
	if err != nil {
		return nil, errors.Tag(err, "cannot get doc")
	}

	resp := &types.ProjectDocMetadataResponse{
		DocId:              docId,
		ProjectDocMetadata: inflate(m.parseDoc(d.Snapshot)),
	}

	if !request.Broadcast {
		// Skip pub/sub for projects with a single active user.
		return resp, nil
	}

	blob, err := json.Marshal(resp)
	if err != nil {
		return nil, errors.Tag(err, "cannot serialize meta")
	}
	err = m.c.Publish(ctx, &sharedTypes.EditorEventsMessage{
		RoomId:  projectId,
		Message: "broadcastDocMeta",
		Payload: blob,
	})
	if err != nil {
		return nil, errors.Tag(err, "cannot publish meta")
	}
	return nil, nil
}

func (m *manager) getForProjectWithoutCache(ctx context.Context, projectId edgedb.UUID, recentlyEdited []*documentUpdaterTypes.DocContentSnapshot) (types.LightProjectMetadata, error) {
	t, err := m.pm.GetProjectWithContent(ctx, projectId)
	if err != nil {
		return nil, errors.Tag(err, "cannot get docs from mongo")
	}
	meta := make(types.LightProjectMetadata, len(recentlyEdited))
	for _, d := range recentlyEdited {
		meta[d.Id.String()] = m.parseDoc(d.Snapshot)
	}
	err = t.WalkDocs(func(e project.TreeElement, path sharedTypes.PathName) error {
		d := e.(*project.Doc)
		id := d.Id.String()
		// Skip parsing of the flushed state if the doc has been edited.
		if _, exists := meta[id]; !exists {
			meta[id] = m.parseDoc(d.Snapshot)
		}
		return nil
	})
	return meta, err
}
