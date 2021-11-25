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

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) deleteFileFromProject(ctx context.Context, projectId primitive.ObjectID, v sharedTypes.Version, mongoPath project.MongoPath, fileRef *project.FileRef) error {
	if err := m.dfm.Create(ctx, projectId, fileRef); err != nil {
		return errors.Tag(err, "cannot create deletedFiles entry")
	}
	err := m.pm.DeleteTreeElement(ctx, projectId, v, mongoPath, fileRef)
	if err != nil {
		return errors.Tag(err, "cannot remove element from tree")
	}
	return nil
}

func (m *manager) DeleteFileFromProject(ctx context.Context, request *types.DeleteFileRequest) error {
	projectId := request.ProjectId
	fileId := request.FileId

	var projectVersion sharedTypes.Version

	err := m.txWithRetries(ctx, func(sCtx context.Context) error {
		t, v, err := m.pm.GetProjectRootFolder(sCtx, projectId)
		if err != nil {
			return errors.Tag(err, "cannot get project")
		}

		var fileRef *project.FileRef
		var mongoPath project.MongoPath
		err = t.WalkFilesMongo(func(_ *project.Folder, f project.TreeElement, fsPath sharedTypes.PathName, mPath project.MongoPath) error {
			if f.GetId() == fileId {
				fileRef = f.(*project.FileRef)
				mongoPath = mPath
				return project.AbortWalk
			}
			return nil
		})
		if err != nil {
			return err
		}
		if fileRef == nil {
			return errors.Tag(&errors.NotFoundError{}, "unknown fileId")
		}
		err = m.deleteFileFromProject(ctx, projectId, v, mongoPath, fileRef)
		if err != nil {
			return err
		}
		projectVersion = v + 1
		return nil
	})
	if err != nil {
		return err
	}

	// The file has been deleted.
	// Failing the request and retrying now would result in a 404.
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	{
		// Notify real-time
		source := "editor"
		payload := []interface{}{fileId, source, projectVersion}
		if b, err2 := json.Marshal(payload); err2 == nil {
			_ = m.editorEvents.Publish(ctx, &sharedTypes.EditorEventsMessage{
				RoomId:  projectId,
				Message: "removeEntity",
				Payload: b,
			})
		}
	}
	return nil
}
