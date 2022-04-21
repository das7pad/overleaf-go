// Golang port of Overleaf
// Copyright (C) 2021-2022 Jakob Ackermann <das7pad@outlook.com>
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

	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) DeleteFolderFromProject(ctx context.Context, request *types.DeleteFolderRequest) error {
	projectId := request.ProjectId
	userId := request.UserId
	folderId := request.FolderId

	projectVersion, err := m.pm.DeleteFolder(ctx, projectId, userId, folderId)
	if err != nil {
		return err
	}

	// The folder has been deleted.
	{
		// Notify real-time first, triggering users to leave child docs.
		source := "editor"
		m.notifyEditor(
			projectId, "removeEntity",
			folderId, source, projectVersion,
		)
	}
	// Failing the request and retrying now would result in a 404.
	ctx, done := context.WithTimeout(context.Background(), 20*time.Second)
	defer done()
	{
		// Cleanup in document-updater
		// Do this in bulk, which does one call to get the list of actually
		//  loaded docIds.
		_ = m.dum.FlushAndDeleteProject(ctx, projectId)
	}
	return nil
}
