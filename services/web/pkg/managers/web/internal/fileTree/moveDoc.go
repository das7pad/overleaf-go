// Golang port of Overleaf
// Copyright (C) 2021-2023 Jakob Ackermann <das7pad@outlook.com>
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

	documentUpdaterTypes "github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) MoveDocInProject(ctx context.Context, request *types.MoveDocRequest) error {
	projectId := request.ProjectId
	userId := request.UserId
	targetFolderId := request.TargetFolderId
	docId := request.DocId

	projectVersion, newFsPath, err := m.pm.MoveDoc(
		ctx, projectId, userId, targetFolderId, docId,
	)
	if err != nil {
		return err
	}

	// The doc has been moved.
	// Failing the request and retrying now would result in duplicate updates.
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	{
		updates := []documentUpdaterTypes.RenameDocUpdate{
			{
				DocId:   docId,
				NewPath: newFsPath,
			},
		}
		_ = m.dum.ProcessProjectUpdates(ctx, projectId, updates)
	}

	m.notifyEditor(projectId, "receiveEntityMove", moveTreeElementUpdate{
		EntityId:       docId,
		TargetFolderId: targetFolderId,
		ProjectVersion: projectVersion,
	})
	return nil
}
