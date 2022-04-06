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

package projectInvite

import (
	"context"

	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/projectInvite"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) AcceptProjectInvite(ctx context.Context, request *types.AcceptProjectInviteRequest, response *types.AcceptProjectInviteResponse) error {
	if err := request.Session.CheckIsLoggedIn(); err != nil {
		return err
	}
	if err := request.Token.Validate(); err != nil {
		return err
	}
	projectId := request.ProjectId
	userId := request.Session.User.Id

	for i := 0; i < 10; i++ {
		d, err := m.pm.GetAuthorizationDetails(ctx, projectId, userId, "")
		if err != nil && !errors.IsNotAuthorizedError(err) {
			return err
		}
		epoch := d.Epoch

		pi := &projectInvite.WithoutToken{}
		err = m.pim.GetByToken(ctx, projectId, request.Token, pi)
		if err != nil {
			return errors.Tag(err, "cannot get invite")
		}
		level := pi.PrivilegeLevel
		invitingUserId := pi.SendingUser.Id
		grantAccess :=
			!(d.PrivilegeLevel.IsAtLeast(level) && d.IsTokenMember == false)

		// TODO: merge into a single query
		err = m.c.Tx(ctx, func(ctx context.Context, _ *edgedb.Tx) error {
			if grantAccess {
				err = m.pm.GrantMemberAccess(
					ctx, projectId, epoch, userId, level,
				)
				if err != nil {
					return errors.Tag(err, "cannot grant access")
				}
			}

			// The contact hints are transparent to the user and OK to fail.
			// TODO: consider merging into m.pim.Delete
			_ = m.um.AddContact(ctx, invitingUserId, userId)

			if err = m.pim.Delete(ctx, projectId, pi.Id); err != nil {
				return errors.Tag(err, "cannot delete invite")
			}
			return nil
		})
		if err != nil {
			if errors.GetCause(err) == project.ErrEpochIsNotStable {
				continue
			}
			return err
		}

		go m.notifyEditorAboutChanges(projectId, &refreshMembershipDetails{
			Invites: true,
			Members: true,
		})

		response.RedirectTo = "/project/" + projectId.String()
		return nil
	}
	return project.ErrEpochIsNotStable
}
