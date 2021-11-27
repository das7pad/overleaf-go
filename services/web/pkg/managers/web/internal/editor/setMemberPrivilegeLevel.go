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

package editor

import (
	"context"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/jwt/projectJWT"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

var errUserIsNotAMember = errors.Tag(&errors.NotFoundError{}, "user is not a member")

func (m *manager) SetMemberPrivilegeLevelInProject(ctx context.Context, request *types.SetMemberPrivilegeLevelInProjectRequest) error {
	if err := request.PrivilegeLevel.Validate(); err != nil {
		return err
	}
	projectId := request.ProjectId
	userId := request.UserId
	for i := 0; i < 10; i++ {
		d, err := m.pm.GetAuthorizationDetails(ctx, projectId, userId, "")
		if err != nil {
			if errors.IsNotAuthorizedError(err) {
				return errUserIsNotAMember
			}
			return errors.Tag(err, "cannot get project")
		}
		if d.IsTokenMember {
			return errUserIsNotAMember
		}

		// Clearing the epoch is OK to do at any time.
		// Clear it just ahead of changing access.
		err = projectJWT.ClearProjectField(ctx, m.client, projectId)
		if err != nil {
			return err
		}
		err = m.pm.GrantMemberAccess(
			ctx, projectId, d.Epoch, userId, request.PrivilegeLevel,
		)
		// Clear it immediately after changing access.
		_ = projectJWT.ClearProjectField(ctx, m.client, projectId)
		if err != nil {
			if errors.GetCause(err) == project.ErrEpochIsNotStable {
				continue
			}
			return errors.Tag(err, "cannot remove user from project")
		}

		go m.notifyEditorAboutAccessChanges(projectId, &refreshMembershipDetails{
			Members: true,
		})
		return nil
	}
	return project.ErrEpochIsNotStable
}
