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

	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) MoveFileInProject(ctx context.Context, request *types.MoveFileRequest) error {
	projectId := request.ProjectId
	userId := request.UserId
	targetFolderId := request.TargetFolderId
	fileId := request.FileId

	projectVersion, _, err := m.pm.MoveFile(
		ctx, projectId, userId, targetFolderId, fileId,
	)
	if err != nil {
		return err
	}

	{
		// Notify real-time
		//goland:noinspection SpellCheckingInspection
		m.notifyEditor(
			projectId, "reciveEntityMove",
			fileId, targetFolderId, projectVersion,
		)
	}
	return nil
}
