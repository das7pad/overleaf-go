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
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) DeleteProject(ctx context.Context, request *types.DeleteProjectRequest) error {
	return m.DeleteProjectInTx(ctx, ctx, request)
}

func (m *manager) DeleteProjectInTx(ctx, sCtx context.Context, request *types.DeleteProjectRequest) error {
	if err := request.Session.CheckIsLoggedIn(); err != nil {
		return err
	}
	ipAddress := request.IPAddress
	userId := request.Session.User.Id
	projectId := request.ProjectId

	{
		d, err := m.pm.GetAuthorizationDetails(sCtx, projectId, userId, "")
		if err != nil {
			return errors.Tag(err, "cannot get project")
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
		return errors.Tag(err, "cannot flush project")
	}

	if err := m.pm.SoftDelete(sCtx, projectId, userId, ipAddress); err != nil {
		return errors.Tag(err, "cannot delete project")
	}
	return nil
}
