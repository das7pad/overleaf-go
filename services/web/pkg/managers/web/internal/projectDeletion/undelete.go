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

package projectDeletion

import (
	"context"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) UnDeleteProject(ctx context.Context, request *types.UnDeleteProjectRequest) error {
	if err := request.Session.CheckIsLoggedIn(); err != nil {
		return err
	}
	userId := request.Session.User.Id
	projectId := request.ProjectId

	name, err := m.pm.GetDeletedProjectsName(ctx, projectId, userId)
	if err != nil {
		return err
	}
	names, err := m.pm.GetProjectNames(ctx, userId)
	if err != nil {
		return errors.Tag(err, "cannot get other project names")
	}
	name = names.MakeUnique(name)

	if err = m.pm.Restore(ctx, projectId, userId, name); err != nil {
		return errors.Tag(err, "cannot restore project")
	}
	return nil
}
