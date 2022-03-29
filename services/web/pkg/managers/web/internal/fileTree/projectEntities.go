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
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) GetProjectEntities(ctx context.Context, request *types.GetProjectEntitiesRequest, response *types.GetProjectEntitiesResponse) error {
	if err := request.Session.CheckIsLoggedIn(); err != nil {
		return err
	}

	userId := request.Session.User.Id
	p, err := m.pm.GetEntries(ctx, request.ProjectId, userId)
	if err != nil {
		return errors.Tag(err, "cannot get project")
	}

	nDocs := len(p.Docs)
	entities := make([]types.GetProjectEntitiesEntry, nDocs+len(p.Files))
	for i, doc := range p.Docs {
		entities[i].Path = string("/" + doc.GetPath())
		entities[i].Type = "doc"
	}
	for i, file := range p.Files {
		entities[nDocs+i].Path = string("/" + file.GetPath())
		entities[nDocs+i].Type = "file"
	}
	response.Entities = entities
	return nil
}
