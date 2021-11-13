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

func (m *manager) RenameFileInProject(ctx context.Context, request *types.RenameFileRequest) error {
	if err := request.Name.Validate(); err != nil {
		return err
	}
	projectId := request.ProjectId
	fileRef := &project.FileRef{}
	fileRef.Id = request.FileId
	fileRef.Name = request.Name

	r, err := m.rename(ctx, projectId, fileRef)
	if err != nil {
		return ignoreAlreadyRenamedErr(err)
	}
	fileRef = r.element.(*project.FileRef)
	oldFsPath := r.oldFsPath.(sharedTypes.PathName)
	newFsPath := r.newFsPath.(sharedTypes.PathName)

	// The file has been renamed.
	// Failing the request and retrying now would result in duplicate updates.
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	{
		// Notify document-updater
		n := &documentUpdaterTypes.ProcessProjectUpdatesRequest{
			ProjectVersion: r.projectVersion,
			Updates: []*documentUpdaterTypes.GenericProjectUpdate{
				documentUpdaterTypes.NewRenameFileUpdate(
					fileRef.GetId(),
					oldFsPath,
					newFsPath,
				).ToGeneric(),
			},
		}
		_ = m.dum.ProcessProjectUpdates(ctx, projectId, n)
	}
	{
		// Notify real-time
		payload := []interface{}{fileRef.Id, fileRef.Name}
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
