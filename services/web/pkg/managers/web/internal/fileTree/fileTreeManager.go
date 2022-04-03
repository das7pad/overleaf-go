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

package fileTree

import (
	"context"
	"encoding/json"
	"time"

	"github.com/edgedb/edgedb-go"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/pubSub/channel"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/docstore/pkg/managers/docstore"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater"
	"github.com/das7pad/overleaf-go/services/filestore/pkg/managers/filestore"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/projectMetadata"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	AddDocToProject(ctx context.Context, request *types.AddDocRequest, response *types.AddDocResponse) error
	AddFolderToProject(ctx context.Context, request *types.AddFolderRequest, response *types.AddFolderResponse) error
	DeleteDocFromProject(ctx context.Context, request *types.DeleteDocRequest) error
	DeleteFileFromProject(ctx context.Context, request *types.DeleteFileRequest) error
	DeleteFolderFromProject(ctx context.Context, request *types.DeleteFolderRequest) error
	GetProjectEntities(ctx context.Context, request *types.GetProjectEntitiesRequest, response *types.GetProjectEntitiesResponse) error
	MoveDocInProject(ctx context.Context, request *types.MoveDocRequest) error
	MoveFileInProject(ctx context.Context, request *types.MoveFileRequest) error
	MoveFolderInProject(ctx context.Context, request *types.MoveFolderRequest) error
	RenameDocInProject(ctx context.Context, request *types.RenameDocRequest) error
	RenameFileInProject(ctx context.Context, request *types.RenameFileRequest) error
	RenameFolderInProject(ctx context.Context, request *types.RenameFolderRequest) error
	RestoreDeletedDocInProject(ctx context.Context, request *types.RestoreDeletedDocRequest, response *types.RestoreDeletedDocResponse) error
	UploadFile(ctx context.Context, request *types.UploadFileRequest) error
}

func New(c *edgedb.Client, db *mongo.Database, pm project.Manager, dm docstore.Manager, dum documentUpdater.Manager, fm filestore.Manager, editorEvents channel.Writer, pmm projectMetadata.Manager) Manager {
	return &manager{
		c:               c,
		db:              db,
		dm:              dm,
		dum:             dum,
		editorEvents:    editorEvents,
		fm:              fm,
		pm:              pm,
		projectMetadata: pmm,
	}
}

type manager struct {
	c               *edgedb.Client
	db              *mongo.Database
	dm              docstore.Manager
	dum             documentUpdater.Manager
	editorEvents    channel.Writer
	fm              filestore.Manager
	options         *types.Options
	pm              project.Manager
	projectMetadata projectMetadata.Manager
}

func (m *manager) notifyEditor(projectId edgedb.UUID, message string, args ...interface{}) {
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	blob, err := json.Marshal(args)
	if err != nil {
		return
	}
	_ = m.editorEvents.Publish(ctx, &sharedTypes.EditorEventsMessage{
		RoomId:  projectId,
		Message: message,
		Payload: blob,
	})
}
