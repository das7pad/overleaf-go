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

func (m *manager) RenameDocInProject(ctx context.Context, request *types.RenameDocRequest) error {
	if err := request.Name.Validate(); err != nil {
		return err
	}
	projectId := request.ProjectId
	docId := request.DocId
	userId := request.UserId
	name := request.Name

	var oldFsPath sharedTypes.PathName
	var newFsPath sharedTypes.PathName
	var doc *project.Doc
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

		var parent *project.Folder
		var mongoPath project.MongoPath
		err = t.WalkDocsMongo(func(f *project.Folder, element project.TreeElement, path sharedTypes.PathName, mPath project.MongoPath) error {
			if element.GetId() == docId {
				parent = f
				doc = element.(*project.Doc)
				oldFsPath = path
				mongoPath = mPath
				return project.AbortWalk
			}
			return nil
		})
		if err != nil {
			return err
		}
		if doc == nil {
			return errors.Tag(&errors.NotFoundError{}, "unknown docId")
		}
		if doc.Name == name {
			// Already renamed.
			return nil
		}

		if err = parent.CheckIsUniqueName(name); err != nil {
			return err
		}

		doc.Name = name
		newFsPath = oldFsPath.Dir().Join(name)
		err = m.pm.RenameTreeElement(ctx, projectId, p.Version, mongoPath, doc)
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

	// The doc has been renamed.
	// Failing the request and retrying now would result in duplicate updates.
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	{
		// Notify document-updater
		r := &documentUpdaterTypes.ProcessProjectUpdatesRequest{
			ProjectVersion: projectVersion,
			Updates: []*documentUpdaterTypes.GenericProjectUpdate{
				documentUpdaterTypes.NewRenameDocUpdate(
					doc.GetId(),
					oldFsPath,
					newFsPath,
				).ToGeneric(),
			},
		}
		_ = m.dum.ProcessProjectUpdates(ctx, projectId, r)
	}
	{
		// Notify real-time
		payload := []interface{}{doc.Id, name}
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
