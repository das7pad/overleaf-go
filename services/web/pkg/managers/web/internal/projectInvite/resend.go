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

	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/projectInvite"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) ResendProjectInvite(ctx context.Context, request *types.ResendProjectInviteRequest) error {
	projectId := request.ProjectId
	senderUserId := request.SenderUserId
	inviteId := request.InviteId

	pi := &projectInvite.WithToken{}
	if err := m.pim.GetById(ctx, projectId, inviteId, pi); err != nil {
		return errors.Tag(err, "cannot get invite")
	}

	p := &project.WithIdAndName{}
	s := &user.WithPublicInfo{}
	u := &user.WithPublicInfo{}

	eg, pCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		if err := m.pm.GetProject(pCtx, projectId, p); err != nil {
			return errors.Tag(err, "cannot get project")
		}
		return nil
	})

	eg.Go(func() error {
		if err := m.um.GetUser(pCtx, senderUserId, s); err != nil {
			return errors.Tag(err, "cannot get sender details")
		}
		return nil
	})

	eg.Go(func() error {
		err := m.um.GetUserByEmail(pCtx, pi.Email, u)
		if err == nil {
			// cleanup any auto-generated first name
			// Side note: The email is normalized to lower case, which reduces
			//             the chance for false-positives.
			if u.FirstName == u.Email.LocalPart() {
				u.FirstName = ""
			}
			// Set the primary email field to the invited email address.
			u.Email = pi.Email
			return nil
		} else if errors.IsNotFoundError(err) {
			// Non-registered user, just populate the primary email field.
			u.Email = pi.Email
			return nil
		} else {
			return errors.Tag(err, "cannot get user")
		}
	})

	if err := eg.Wait(); err != nil {
		return err
	}

	if !u.Id.IsZero() {
		if err := m.createNotification(ctx, p, s, u, pi); err != nil {
			return errors.Tag(err, "cannot create notification")
		}
	}

	if err := m.sendEmail(ctx, p, pi, s, u); err != nil {
		return err
	}
	return nil
}
