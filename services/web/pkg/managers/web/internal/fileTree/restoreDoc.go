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
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) RestoreDeletedDocInProject(ctx context.Context, request *types.RestoreDeletedDocRequest, response *types.RestoreDeletedDocResponse) error {
	if err := request.Name.Validate(); err != nil {
		return err
	}
	projectId := request.ProjectId

	d, errGetDoc := m.dm.GetFullDoc(ctx, projectId, request.DocId)
	if errGetDoc != nil {
		return errors.Tag(errGetDoc, "cannot get doc")
	}
	if !d.Deleted {
		return &errors.InvalidStateError{Msg: "doc is not deleted"}
	}
	doc := project.NewDoc(request.Name)

	var rootFolderId primitive.ObjectID
	var projectVersion sharedTypes.Version

	err := m.txWithRetries(ctx, func(ctx context.Context) error {
		t, v, err := m.pm.GetProjectRootFolder(ctx, projectId)
		if err != nil {
			return errors.Tag(err, "cannot get project")
		}
		rootFolderId = t.Id

		name := request.Name
		if t.HasEntry(name) {
			ext := sharedTypes.PathName(name).Type()
			base := name.Basename()
			if len(ext) > 10 {
				ext = ""
				base = string(name)
			}
			if ext != "" {
				ext = "." + ext
			}
			now := time.Now().UTC().Format(time.RFC3339)
			if len(base) > 150-11-15-21-len(ext) {
				// maxLen - truncated - restoredOnLen - timeLen - extLen
				// 150    - 11        - 15            - 21      - ?
				base = base[:150-11-15-21-len(ext)] + "[truncated]"
			}
			name = sharedTypes.Filename(fmt.Sprintf(
				"%s (Restored on %s)%s", base, now, ext,
			))
		}
		doc.Name = name

		var path project.MongoPath
		_ = t.WalkFoldersMongo(func(_, f *project.Folder, _ sharedTypes.DirName, mongoPath project.MongoPath) error {
			if f.Id == rootFolderId {
				path = mongoPath
				return project.AbortWalk
			}
			return nil
		})
		if path == "" {
			return errors.New("cannot find rootFolder")
		}
		err = m.dm.CreateDocWithContent(
			ctx, projectId, doc.Id, d.Lines.ToSnapshot(),
		)
		if err != nil {
			return errors.Tag(err, "cannot create doc")
		}
		err = m.pm.AddTreeElement(ctx, projectId, v, path, doc)
		if err != nil {
			return errors.Tag(err, "cannot add doc")
		}
		projectVersion = v + 1
		return nil
	})
	if err != nil {
		return err
	}

	response.DocId = d.Id

	//goland:noinspection SpellCheckingInspection
	go m.notifyEditor(
		projectId, "reciveNewDoc",
		rootFolderId, doc, projectVersion,
	)
	return nil
}
