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

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) AddFolderToProject(ctx context.Context, request *types.AddFolderRequest, response *types.AddFolderResponse) error {
	if err := request.Name.Validate(); err != nil {
		return err
	}
	projectId := request.ProjectId
	parentFolderId := request.ParentFolderId
	name := request.Name

	folder := project.NewFolder(name)
	folderId, err := sharedTypes.GenerateUUID()
	if err != nil {
		return err
	}
	folder.Id = folderId
	projectVersion, err := m.pm.AddFolder(
		ctx, projectId, request.UserId, parentFolderId, &folder,
	)
	if err != nil {
		return errors.Tag(err, "cannot insert folder")
	}

	*response = folder

	//goland:noinspection SpellCheckingInspection
	m.notifyEditor(
		projectId, "reciveNewFolder", parentFolderId, folder, projectVersion,
	)
	return nil
}
