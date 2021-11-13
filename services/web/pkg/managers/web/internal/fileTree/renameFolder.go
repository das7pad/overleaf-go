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

	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	documentUpdaterTypes "github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) RenameFolderInProject(ctx context.Context, request *types.RenameFolderRequest) error {
	if err := request.Name.Validate(); err != nil {
		return err
	}
	projectId := request.ProjectId
	folder := &project.Folder{}
	folder.Id = request.FolderId
	folder.Name = request.Name

	r, err := m.rename(ctx, projectId, folder)
	if err != nil {
		return ignoreAlreadyRenamedErr(err)
	}
	folder = r.element.(*project.Folder)
	oldFsPath := r.oldFsPath.(sharedTypes.DirName)
	newFsPath := r.newFsPath.(sharedTypes.DirName)

	// The folder has been renamed.
	// Failing the request and retrying now would result in duplicate updates.
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	{
		// Notify document-updater
		updates := make([]*documentUpdaterTypes.GenericProjectUpdate, 0)
		err2 := folder.WalkDocs(
			func(e project.TreeElement, p sharedTypes.PathName) error {
				updates = append(
					updates,
					documentUpdaterTypes.NewRenameDocUpdate(
						e.GetId(),
						oldFsPath.JoinPath(p),
						newFsPath.JoinPath(p),
					).ToGeneric(),
				)
				return nil
			},
		)
		if err2 == nil && len(updates) > 0 {
			p := &documentUpdaterTypes.ProcessProjectUpdatesRequest{
				ProjectVersion: r.projectVersion,
				Updates:        updates,
			}
			_ = m.dum.ProcessProjectUpdates(ctx, projectId, p)
		}
	}
	{
		// Notify real-time
		payload := []interface{}{folder.Id, folder.Name, r.projectVersion}
		if b, err2 := json.Marshal(payload); err2 == nil {
			//goland:noinspection SpellCheckingInspection
			_ = m.editorEvents.Publish(ctx, &sharedTypes.EditorEventsMessage{
				RoomId:  projectId,
				Message: "reciveEntityRename",
				Payload: b,
			})
		}
	}
	return nil
}
