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
	docModel "github.com/das7pad/overleaf-go/pkg/models/doc"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) markDocAsDeleted(ctx context.Context, projectId primitive.ObjectID, doc *project.Doc) error {
	meta := docModel.Meta{}
	meta.Deleted = true
	meta.DeletedAt = time.Now().UTC()
	meta.Name = doc.Name
	if err := m.dm.PatchDoc(ctx, projectId, doc.Id, meta); err != nil {
		return errors.Tag(err, "cannot delete doc")
	}
	return nil
}

func (m *manager) deleteDocFromProject(ctx context.Context, projectId primitive.ObjectID, v sharedTypes.Version, rootDocId primitive.ObjectID, mongoPath project.MongoPath, doc *project.Doc) error {
	if err := m.markDocAsDeleted(ctx, projectId, doc); err != nil {
		return errors.Tag(err, "cannot delete doc")
	}

	var err error
	if doc.Id == rootDocId {
		err = m.pm.DeleteTreeElementAndRootDoc(ctx, projectId, v, mongoPath, doc)
	} else {
		err = m.pm.DeleteTreeElement(ctx, projectId, v, mongoPath, doc)
	}
	if err != nil {
		return errors.Tag(err, "cannot remove element from tree")
	}
	return nil
}

func (m *manager) DeleteDocFromProject(ctx context.Context, request *types.DeleteDocRequest) error {
	projectId := request.ProjectId
	docId := request.DocId

	var projectVersion sharedTypes.Version

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

		var doc *project.Doc
		var mongoPath project.MongoPath
		err = t.WalkDocsMongo(func(_ *project.Folder, d project.TreeElement, fsPath sharedTypes.PathName, mPath project.MongoPath) error {
			if d.GetId() == docId {
				doc = d.(*project.Doc)
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

		err = m.deleteDocFromProject(
			ctx, projectId, v, p.RootDocId, mongoPath, doc,
		)
		if err != nil {
			return err
		}
		projectVersion = v + 1
		return nil
	})
	if err != nil {
		return err
	}

	// The doc has been deleted.
	// Failing the request and retrying now would result in a 404.
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	{
		// Notify real-time first, triggering users to leave the doc.
		source := "editor"
		payload := []interface{}{docId, source, projectVersion}
		if b, err2 := json.Marshal(payload); err2 == nil {
			_ = m.editorEvents.Publish(ctx, &sharedTypes.EditorEventsMessage{
				RoomId:  projectId,
				Message: "removeEntity",
				Payload: b,
			})
		}
	}
	m.cleanupDocDeletion(ctx, projectId, docId)
	return nil
}

func (m *manager) cleanupDocDeletion(ctx context.Context, projectId, docId primitive.ObjectID) {
	// Cleanup in document-updater
	_ = m.dum.FlushAndDeleteDoc(ctx, projectId, docId)
	// Bonus: Archive the doc
	_ = m.dm.ArchiveDoc(ctx, projectId, docId)
}
