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

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) GetProjectEntities(ctx context.Context, request *types.GetProjectEntitiesRequest, response *types.GetProjectEntitiesResponse) error {
	if err := request.Session.CheckIsLoggedIn(); err != nil {
		return err
	}

	userId := request.Session.User.Id
	p := &project.WithTreeAndAuth{}
	if err := m.pm.GetProject(ctx, request.ProjectId, p); err != nil {
		return errors.Tag(err, "cannot get project")
	}
	if _, err := p.GetPrivilegeLevelAuthenticated(userId); err != nil {
		return err
	}

	f, err := p.GetRootFolder()
	if err != nil {
		return err
	}
	entities := make([]types.GetProjectEntitiesEntry, 0)
	err = f.Walk(func(e project.TreeElement, path sharedTypes.PathName) error {
		switch e.(type) {
		case project.Doc:
			entities = append(entities, types.GetProjectEntitiesEntry{
				Path: "/" + path,
				Type: "doc",
			})
		case project.FileRef:
			entities = append(entities, types.GetProjectEntitiesEntry{
				Path: "/" + path,
				Type: "file",
			})
		}
		return nil
	})
	if err != nil {
		return err
	}
	response.Entities = entities
	return nil
}
