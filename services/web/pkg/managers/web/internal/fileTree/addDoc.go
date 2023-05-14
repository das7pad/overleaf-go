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

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) AddDocToProject(ctx context.Context, request *types.AddDocRequest, response *types.AddDocResponse) error {
	if err := request.Validate(); err != nil {
		return err
	}
	projectId := request.ProjectId
	parentFolderId := request.ParentFolderId
	name := request.Name

	doc := project.NewDoc(name)
	if err := sharedTypes.PopulateUUID(&doc.Id); err != nil {
		return err
	}
	projectVersion, err := m.pm.CreateDoc(
		ctx, request.ProjectId, request.UserId, request.ParentFolderId, &doc,
	)
	if err != nil {
		return errors.Tag(err, "create doc")
	}
	*response = doc

	m.notifyEditor(projectId, "receiveNewDoc", newTreeElementUpdate{
		Doc:            &doc,
		ParentFolderId: parentFolderId,
		ProjectVersion: projectVersion,
		ClientId:       request.ClientId,
	})
	return nil
}
