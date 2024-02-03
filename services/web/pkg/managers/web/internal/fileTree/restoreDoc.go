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

	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) RestoreDeletedDocInProject(ctx context.Context, request *types.RestoreDeletedDocRequest, response *types.RestoreDeletedDocResponse) error {
	if err := request.Name.Validate(); err != nil {
		return err
	}
	projectId := request.ProjectId

	projectVersion, rootFolderId, err := m.pm.RestoreDoc(
		ctx, projectId, request.UserId, request.DocId, request.Name,
	)
	if err != nil {
		return err
	}
	response.DocId = request.DocId

	d := project.NewDoc(request.Name)
	d.Id = request.DocId

	m.notifyEditor(projectId, sharedTypes.ReceiveNewDoc, newTreeElementUpdate{
		Doc:            &d,
		ParentFolderId: rootFolderId,
		ProjectVersion: projectVersion,
	})
	return nil
}
