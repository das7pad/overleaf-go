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

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/jwt/projectJWT"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/mongoTx"
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

		pi, err := m.pim.Get(ctx, projectId, request.Token)
		if err != nil {
			return errors.Tag(err, "cannot get invite")
		}
		level := pi.PrivilegeLevel
		invitingUserId := pi.SendingUserId
		grantAccess :=
			!(d.PrivilegeLevel.IsAtLeast(level) && d.IsTokenMember == false)

		err = mongoTx.For(m.db, ctx, func(ctx context.Context) error {
			if grantAccess {
				err = m.pm.GrantMemberAccess(
					ctx, projectId, epoch, userId, level,
				)
				if err != nil {
					return errors.Tag(err, "cannot grant access")
				}
			}

			// The contact hints are transparent to the user and OK to fail.
			_ = m.cm.AddContacts(ctx, invitingUserId, userId)

			if err = m.pim.Delete(ctx, pi.Id); err != nil {
				return errors.Tag(err, "cannot delete invite")
			}

			// While not critical, the UI should rather error out than retain
			//  any stale notifications.
			key := "project-invite-" + pi.Id.Hex()
			if err = m.nm.RemoveNotificationByKeyOnly(ctx, key); err != nil {
				return errors.Tag(err, "cannot delete invite")
			}

			if grantAccess {
				// Clearing the epoch is OK to do at any time.
				// Clear it just ahead of committing the tx.
				err = projectJWT.ClearProjectField(ctx, m.client, projectId)
				if err != nil {
					return err
				}
			}
			return nil
		})
		if grantAccess {
			// Clearing the epoch is OK to do at any time.
			// Clear it immediately after aborting/committing the tx.
			// The user may already be a member by now. Do not error out.
			_ = projectJWT.ClearProjectField(ctx, m.client, projectId)
		}
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

		response.RedirectTo = "/project/" + projectId.Hex()
		return nil
	}
	return project.ErrEpochIsNotStable
}
