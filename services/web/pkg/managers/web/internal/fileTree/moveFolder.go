// Golang port of Overleaf
// Copyright (C) 2021-2024 Jakob Ackermann <das7pad@outlook.com>
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
	"time"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	documentUpdaterTypes "github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) MoveFolderInProject(ctx context.Context, request *types.MoveFolderRequest) error {
	projectId := request.ProjectId
	userId := request.UserId
	targetFolderId := request.TargetFolderId
	folderId := request.FolderId

	projectVersion, docs, _, err := m.pm.MoveFolder(
		ctx, projectId, userId, targetFolderId, folderId,
	)
	if err != nil {
		return err
	}

	// The folder has been moved.
	// Failing the request and retrying now would result in duplicate updates.
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	if len(docs) > 0 {
		updates := make([]documentUpdaterTypes.RenameDocUpdate, len(docs))
		for i, doc := range docs {
			updates[i] = documentUpdaterTypes.RenameDocUpdate{
				DocId:   doc.Id,
				NewPath: doc.Path,
			}
		}
		_ = m.dum.ProcessProjectUpdates(ctx, projectId, updates)
	}

	m.notifyEditor(projectId, sharedTypes.ReceiveEntityMove, moveTreeElementUpdate{
		EntityId:       folderId,
		TargetFolderId: targetFolderId,
		ProjectVersion: projectVersion,
	})
	return nil
}
