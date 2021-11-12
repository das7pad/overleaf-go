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
	"github.com/das7pad/overleaf-go/pkg/mongoTx"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	documentUpdaterTypes "github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) RenameFileInProject(ctx context.Context, request *types.RenameFileRequest) error {
	if err := request.Name.Validate(); err != nil {
		return err
	}
	projectId := request.ProjectId
	fileId := request.FileId
	userId := request.UserId
	name := request.Name

	var oldFsPath sharedTypes.PathName
	var newFsPath sharedTypes.PathName
	var fileRef *project.FileRef
	var projectVersion sharedTypes.Version

	err := mongoTx.For(m.db, ctx, func(sCtx context.Context) error {
		p, err := m.pm.GetTreeAndAuth(sCtx, projectId, userId)
		if err != nil {
			return errors.Tag(err, "cannot get project")
		}
		projectVersion = p.Version

		t, err := p.GetRootFolder()
		if err != nil {
			return err
		}

		var parent *project.Folder
		var mongoPath project.MongoPath
		err = t.WalkFilesMongo(func(f *project.Folder, element project.TreeElement, path sharedTypes.PathName, mPath project.MongoPath) error {
			if element.GetId() == fileId {
				parent = f
				fileRef = element.(*project.FileRef)
				oldFsPath = path
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
		if fileRef.Name == name {
			// Already renamed.
			return errAlreadyRenamed
		}

		if err = parent.CheckIsUniqueName(name); err != nil {
			return err
		}

		fileRef.Name = name
		newFsPath = oldFsPath.Dir().Join(name)
		err = m.pm.RenameTreeElement(sCtx, projectId, p.Version, mongoPath, fileRef)
		if err != nil {
			return errors.Tag(err, "cannot rename element in tree")
		}
		return nil
	})

	if err != nil {
		if err == errAlreadyRenamed {
			return nil
		}
		return err
	}

	// The file has been renamed.
	// Failing the request and retrying now would result in duplicate updates.
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	{
		// Notify document-updater
		r := &documentUpdaterTypes.ProcessProjectUpdatesRequest{
			ProjectVersion: projectVersion,
			Updates: []*documentUpdaterTypes.GenericProjectUpdate{
				documentUpdaterTypes.NewRenameFileUpdate(
					fileRef.GetId(),
					oldFsPath,
					newFsPath,
				).ToGeneric(),
			},
		}
		_ = m.dum.ProcessProjectUpdates(ctx, projectId, r)
	}
	{
		// Notify real-time
		payload := []interface{}{fileRef.Id, name}
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
