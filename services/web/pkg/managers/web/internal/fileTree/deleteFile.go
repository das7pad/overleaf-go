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

	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) DeleteFileFromProject(ctx context.Context, request *types.DeleteFileRequest) error {
	projectId := request.ProjectId
	userId := request.UserId
	fileId := request.FileId

	projectVersion, err := m.pm.DeleteFile(ctx, projectId, userId, fileId)
	if err != nil {
		return err
	}

	{
		// Notify real-time
		source := "editor"
		m.notifyEditor(
			projectId, "removeEntity",
			fileId, source, projectVersion,
		)
	}
	return nil
}
