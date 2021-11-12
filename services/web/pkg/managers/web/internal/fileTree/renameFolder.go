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
	documentUpdaterTypes "github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

var errAlreadyRenamed = &errors.InvalidStateError{Msg: "already renamed"}

func (m *manager) RenameFolderInProject(ctx context.Context, request *types.RenameFolderRequest) error {
	if err := request.Name.Validate(); err != nil {
		return err
	}
	projectId := request.ProjectId
	folderId := request.FolderId
	userId := request.UserId
	name := request.Name

	var oldFsPath sharedTypes.DirName
	var newFsPath sharedTypes.DirName
	var folder *project.Folder
	var projectVersion sharedTypes.Version

	var lastErr error
	for i := 0; i < retriesFileTreeOperation; i++ {
		p, err := m.pm.GetTreeAndAuth(ctx, projectId, userId)
		if err != nil {
			return errors.Tag(err, "cannot get project")
		}
		projectVersion = p.Version

		t, err := p.GetRootFolder()
		if err != nil {
			return err
		}
		if folderId == t.Id {
			return errors.Tag(&errors.NotFoundError{}, "cannot rename rootFolder")
		}

		var parent *project.Folder
		var mongoPath project.MongoPath
		err = t.WalkFoldersMongo(func(p, f *project.Folder, path sharedTypes.DirName, mPath project.MongoPath) error {
			if f.GetId() == folderId {
				parent = p
				folder = f
				oldFsPath = path
				mongoPath = mPath
				return project.AbortWalk
			}
			return nil
		})
		if err != nil {
			return err
		}
		if folder == nil {
			return errors.Tag(&errors.NotFoundError{}, "unknown folderId")
		}
		if folder.Name == name {
			// Already renamed.
			return errAlreadyRenamed
		}

		if err = parent.CheckIsUniqueName(name); err != nil {
			return err
		}

		folder.Name = name
		newFsPath = sharedTypes.DirName(oldFsPath.Dir().Join(name))
		err = m.pm.RenameTreeElement(ctx, projectId, p.Version, mongoPath, folder)
		if err != nil {
			if err == project.ErrVersionChanged {
				lastErr = err
				continue
			}
			return errors.Tag(err, "cannot rename element in tree")
		}
		break
	}
	if lastErr != nil {
		return lastErr
	}

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
						oldFsPath.Join(sharedTypes.Filename(p)),
						newFsPath.Join(sharedTypes.Filename(p)),
					).ToGeneric(),
				)
				return nil
			},
		)
		if err2 == nil {
			r := &documentUpdaterTypes.ProcessProjectUpdatesRequest{
				ProjectVersion: projectVersion,
				Updates:        updates,
			}
			_ = m.dum.ProcessProjectUpdates(ctx, projectId, r)
		}
	}
	{
		// Notify real-time
		payload := []interface{}{folder.Id, name}
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
