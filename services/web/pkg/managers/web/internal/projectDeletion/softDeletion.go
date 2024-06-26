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

package projectDeletion

import (
	"context"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) DeleteUsersOwnedProjects(ctx context.Context, userId sharedTypes.UUID, ipAddress string) error {
	projects, err := m.pm.GetOwnedProjects(ctx, userId)
	if err != nil {
		return errors.Tag(err, "get projects")
	}
	for _, pId := range projects {
		if err = m.dum.FlushAndDeleteProject(ctx, pId); err != nil {
			return errors.Tag(
				err, "flush project "+pId.String(),
			)
		}
	}
	// Only soft delete projects after flushing.
	err = m.pm.SoftDelete(ctx, projects, userId, ipAddress)
	if err != nil {
		return errors.Tag(err, "soft deleted owned projects")
	}
	return nil
}

func (m *manager) DeleteProject(ctx context.Context, request *types.DeleteProjectRequest) error {
	if err := request.Session.CheckIsLoggedIn(); err != nil {
		return err
	}
	ipAddress := request.IPAddress
	userId := request.Session.User.Id
	projectId := request.ProjectId

	{
		d, err := m.pm.GetAuthorizationDetails(ctx, projectId, userId, "")
		if err != nil {
			return errors.Tag(err, "get project")
		}
		if e := request.EpochHint; e != nil && d.Epoch != *e {
			return project.ErrEpochIsNotStable
		}
		err = d.PrivilegeLevel.CheckIsAtLeast(sharedTypes.PrivilegeLevelOwner)
		if err != nil {
			return err
		}
	}
	if err := m.dum.FlushAndDeleteProject(ctx, projectId); err != nil {
		return errors.Tag(err, "flush project")
	}
	projectIds := []sharedTypes.UUID{projectId}
	if err := m.pm.SoftDelete(ctx, projectIds, userId, ipAddress); err != nil {
		return errors.Tag(err, "delete project")
	}
	return nil
}
