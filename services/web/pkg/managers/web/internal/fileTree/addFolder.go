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

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) AddFolderToProject(ctx context.Context, request *types.AddFolderRequest, response *types.AddFolderResponse) error {
	if err := request.Name.Validate(); err != nil {
		return err
	}
	projectId := request.ProjectId
	parentFolderId := request.ParentFolderId
	name := request.Name

	var projectVersion sharedTypes.Version
	folder := project.NewFolder(name)

	var lastErr error
	for i := 0; i < retriesFileTreeOperation; i++ {
		t, v, err := m.pm.GetProjectRootFolder(ctx, projectId)
		if err != nil {
			return errors.Tag(err, "cannot get project")
		}

		var target *project.Folder
		var mongoPath project.MongoPath
		if parentFolderId.IsZero() {
			parentFolderId = t.Id
		}
		err = t.WalkFoldersMongo(func(_, f *project.Folder, _ sharedTypes.DirName, mPath project.MongoPath) error {
			if f.GetId() == parentFolderId {
				target = f
				mongoPath = mPath + ".folders"
				return project.AbortWalk
			}
			return nil
		})
		if err != nil {
			return err
		}
		if target == nil {
			return errors.Tag(&errors.NotFoundError{}, "unknown parentFolderId")
		}

		if err = target.CheckIsUniqueName(name); err != nil {
			return err
		}

		err = m.pm.AddTreeElement(ctx, projectId, v, mongoPath, folder)
		if err != nil {
			if err == project.ErrVersionChanged {
				lastErr = err
				continue
			}
			return errors.Tag(err, "cannot add element into tree")
		}
		projectVersion = v + 1
		break
	}
	if lastErr != nil {
		return lastErr
	}

	*response = *folder

	// The new folder has been created.
	// Failing the request and retrying now would result in duplicates.
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	{
		// Notify real-time
		payload := []interface{}{parentFolderId, folder, projectVersion}
		if b, err2 := json.Marshal(payload); err2 == nil {
			//goland:noinspection SpellCheckingInspection
			_ = m.editorEvents.Publish(ctx, &sharedTypes.EditorEventsMessage{
				RoomId:  projectId,
				Message: "reciveNewFolder",
				Payload: b,
			})
		}
	}
	return nil
}
