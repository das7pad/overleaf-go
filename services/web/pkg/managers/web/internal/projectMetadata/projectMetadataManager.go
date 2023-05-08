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

package projectMetadata

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/pubSub/channel"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater"
	documentUpdaterTypes "github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	BroadcastMetadataForDocFromSnapshot(projectId, docId sharedTypes.UUID, snapshot string) error
	GetMetadataForProject(ctx context.Context, request *types.GetMetadataForProjectRequest, response *types.GetMetadataForProjectResponse) error
	GetMetadataForDoc(ctx context.Context, request *types.GetMetadataForDocRequest, response *types.GetMetadataForDocResponse) error
}

func New(client redis.UniversalClient, editorEvents channel.Writer, pm project.Manager, dum documentUpdater.Manager) Manager {
	return &manager{
		client: client,
		c:      editorEvents,
		pm:     pm,
		dum:    dum,
	}
}

type manager struct {
	client redis.UniversalClient
	c      channel.Writer
	pm     project.Manager
	dum    documentUpdater.Manager
}

func (m *manager) GetMetadataForProject(ctx context.Context, request *types.GetMetadataForProjectRequest, response *types.GetMetadataForProjectResponse) error {
	l, err := m.getForProjectWithCache(ctx, request.ProjectId)
	if err != nil {
		return err
	}
	p := make(types.ProjectMetadata, len(l))
	for id, metadata := range l {
		p[id] = inflate(metadata)
	}
	response.ProjectMetadata = p
	return nil
}

func (m *manager) BroadcastMetadataForDocFromSnapshot(projectId, docId sharedTypes.UUID, snapshot string) error {
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	return m.broadcast(ctx, projectId, docId, inflate(m.parseDoc(snapshot)))
}

func (m *manager) GetMetadataForDoc(ctx context.Context, request *types.GetMetadataForDocRequest, response *types.GetMetadataForDocResponse) error {
	d, err := m.dum.GetDoc(ctx, request.ProjectId, request.DocId, -1)
	if err != nil {
		return errors.Tag(err, "get doc")
	}
	meta := inflate(m.parseDoc(d.Snapshot))

	if !request.Broadcast {
		// Skip pub/sub for projects with a single active user.
		response.DocId = request.DocId
		response.ProjectDocMetadata = &meta
		return nil
	}
	return m.broadcast(ctx, request.ProjectId, request.DocId, meta)
}

func (m *manager) broadcast(ctx context.Context, projectId, docId sharedTypes.UUID, meta types.ProjectDocMetadata) error {
	blob, err := json.Marshal(types.GetMetadataForDocResponse{
		DocId:              docId,
		ProjectDocMetadata: &meta,
	})
	if err != nil {
		return errors.Tag(err, "serialize meta")
	}
	err = m.c.Publish(ctx, &sharedTypes.EditorEventsMessage{
		RoomId:  projectId,
		Message: "broadcastDocMeta",
		Payload: blob,
	})
	if err != nil {
		return errors.Tag(err, "publish meta")
	}
	return nil
}

func (m *manager) getForProjectWithoutCache(ctx context.Context, projectId sharedTypes.UUID, recentlyEdited documentUpdaterTypes.DocContentSnapshots) (types.LightProjectMetadata, error) {
	docs, _, err := m.pm.GetProjectWithContent(ctx, projectId)
	if err != nil {
		return nil, errors.Tag(err, "get docs from db")
	}
	meta := make(types.LightProjectMetadata, len(docs))
	for _, d := range recentlyEdited {
		meta[d.Id.String()] = m.parseDoc(d.Snapshot)
	}
	for _, d := range docs {
		id := d.Id.String()
		// Skip parsing of the flushed state if the doc has been edited.
		if _, exists := meta[id]; !exists {
			meta[id] = m.parseDoc(d.Snapshot)
		}
	}
	return meta, err
}
