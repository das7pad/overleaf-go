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

	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	documentUpdaterTypes "github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) RenameDocInProject(ctx context.Context, request *types.RenameDocRequest) error {
	if err := request.Name.Validate(); err != nil {
		return err
	}
	projectId := request.ProjectId
	d := project.Doc{}
	d.Id = request.DocId
	d.Name = request.Name

	projectVersion, path, err := m.pm.RenameDoc(
		ctx, projectId, request.UserId, &d,
	)
	if err != nil {
		return err
	}

	// The doc has been renamed.
	// Failing the request and retrying now would result in duplicate updates.
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	{
		updates := []documentUpdaterTypes.RenameDocUpdate{
			{
				DocId:   d.Id,
				NewPath: path,
			},
		}
		_ = m.dum.ProcessProjectUpdates(ctx, projectId, updates)
	}

	m.notifyEditor(projectId, sharedTypes.ReceiveEntityRename, renameTreeElementUpdate{
		EntityId:       d.Id,
		Name:           d.Name,
		ProjectVersion: projectVersion,
	})
	return nil
}
