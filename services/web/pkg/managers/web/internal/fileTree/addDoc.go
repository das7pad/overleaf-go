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

func (m *manager) AddDocToProject(ctx context.Context, request *types.AddDocRequest, response *types.AddDocResponse) error {
	if err := request.Name.Validate(); err != nil {
		return err
	}
	projectId := request.ProjectId
	parentFolderId := request.ParentFolderId
	userId := request.UserId
	name := request.Name
	source := "editor"

	var projectVersion sharedTypes.Version
	var docPath sharedTypes.PathName

	doc := project.NewDoc(name)

	err := m.txWithRetries(ctx, func(sCtx context.Context) error {
		p, err := m.pm.GetTreeAndAuth(sCtx, projectId, userId)
		if err != nil {
			return errors.Tag(err, "cannot get project")
		}

		t, err := p.GetRootFolder()
		if err != nil {
			return err
		}

		var target *project.Folder
		var mongoPath project.MongoPath
		if parentFolderId.IsZero() {
			parentFolderId = t.Id
		}
		err = t.WalkFoldersMongo(func(_, f *project.Folder, fPath sharedTypes.DirName, mPath project.MongoPath) error {
			if f.GetId() == parentFolderId {
				target = f
				mongoPath = mPath + ".docs"
				docPath = fPath.Join(name)
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

		if err = m.dm.CreateDoc(sCtx, projectId, doc.Id); err != nil {
			return errors.Tag(err, "cannot create empty doc")
		}
		err = m.pm.AddTreeElement(sCtx, projectId, p.Version, mongoPath, doc)
		if err != nil {
			return errors.Tag(err, "cannot add element into tree")
		}
		projectVersion = p.Version + 1
		return nil
	})

	if err != nil {
		return err
	}

	*response = *doc

	// The new doc has been created.
	// Failing the request and retrying now would result in duplicates.
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	{
		// Notify document-updater
		u := documentUpdaterTypes.NewAddDocUpdate(doc.Id, docPath)
		r := &documentUpdaterTypes.ProcessProjectUpdatesRequest{
			ProjectVersion: projectVersion,
			Updates: []*documentUpdaterTypes.GenericProjectUpdate{
				u.ToGeneric(),
			},
		}
		_ = m.dum.ProcessProjectUpdates(ctx, projectId, r)
	}

	{
		// Notify real-time
		payload := []interface{}{parentFolderId, doc, source, userId}
		if b, err2 := json.Marshal(payload); err2 == nil {
			//goland:noinspection SpellCheckingInspection
			_ = m.editorEvents.Publish(ctx, &sharedTypes.EditorEventsMessage{
				RoomId:  projectId,
				Message: "reciveNewDoc",
				Payload: b,
			})
		}
	}
	return nil
}
