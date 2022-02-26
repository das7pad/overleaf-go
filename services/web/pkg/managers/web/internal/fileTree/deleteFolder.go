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

func (m *manager) DeleteFolderFromProject(ctx context.Context, request *types.DeleteFolderRequest) error {
	projectId := request.ProjectId
	folderId := request.FolderId

	var projectVersion sharedTypes.Version
	var folder *project.Folder

	err := m.txWithRetries(ctx, func(sCtx context.Context) error {
		p := &project.WithTreeAndRootDoc{}
		if err := m.pm.GetProject(sCtx, projectId, p); err != nil {
			return errors.Tag(err, "cannot get project")
		}
		v := p.Version
		t, err := p.GetRootFolder()
		if err != nil {
			return err
		}

		var mongoPath project.MongoPath
		err = t.WalkFoldersMongo(func(_, f *project.Folder, fsPath sharedTypes.DirName, mPath project.MongoPath) error {
			if f.Id == folderId {
				folder = f
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

		folderContainsRootDoc := false
		err = folder.WalkDocs(func(e project.TreeElement, _ sharedTypes.PathName) error {
			doc := e.(*project.Doc)
			if doc.Id == p.RootDocId {
				folderContainsRootDoc = true
			}
			if err2 := m.markDocAsDeleted(sCtx, projectId, doc); err2 != nil {
				return errors.Tag(err2, "child docId: "+doc.Id.String())
			}
			return nil
		})
		if err != nil {
			return err
		}

		fileRefs := make([]*project.FileRef, 0)
		err = folder.WalkFiles(func(e project.TreeElement, _ sharedTypes.PathName) error {
			fileRef := e.(*project.FileRef)
			fileRefs = append(fileRefs, fileRef)
			return nil
		})
		if err != nil {
			return err
		}
		if len(fileRefs) > 0 {
			if err = m.dfm.CreateBulk(sCtx, projectId, fileRefs); err != nil {
				return errors.Tag(err, "cannot create deletedFiles entries")
			}
		}

		if folderContainsRootDoc {
			err = m.pm.DeleteTreeElementAndRootDoc(sCtx, projectId, v, mongoPath, folder)
		} else {
			err = m.pm.DeleteTreeElement(sCtx, projectId, v, mongoPath, folder)
		}
		if err != nil {
			return errors.Tag(err, "cannot remove element from tree")
		}
		projectVersion = v + 1
		return nil
	})
	if err != nil {
		return err
	}

	// The doc has been deleted.
	// Failing the request and retrying now would result in a 404.
	ctx, done := context.WithTimeout(context.Background(), 20*time.Second)
	defer done()
	{
		// Notify real-time first, triggering users to leave the doc.
		source := "editor"
		payload := []interface{}{folderId, source, projectVersion}
		if b, err2 := json.Marshal(payload); err2 == nil {
			_ = m.editorEvents.Publish(ctx, &sharedTypes.EditorEventsMessage{
				RoomId:  projectId,
				Message: "removeEntity",
				Payload: b,
			})
		}
	}
	{
		// Cleanup in document-updater
		// Do this in bulk, which does one call to get the list of actually
		//  loaded docIds.
		_ = m.dum.FlushAndDeleteProject(ctx, projectId)
	}
	{
		// Bonus: Archive the doc
		_ = folder.WalkDocs(func(d project.TreeElement, _ sharedTypes.PathName) error {
			return m.dm.ArchiveDoc(ctx, projectId, d.GetId())
		})
	}
	return nil
}
