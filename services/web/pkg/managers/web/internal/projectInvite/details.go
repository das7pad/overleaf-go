// Golang port of Overleaf
// Copyright (C) 2021-2022 Jakob Ackermann <das7pad@outlook.com>
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
	"fmt"
	"net/url"

	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/projectInvite"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type projectInviteDetails struct {
	project *project.ForProjectInvite
	invite  *projectInvite.WithToken
	sender  *user.WithPublicInfo
	user    *user.WithPublicInfo
}

func (d *projectInviteDetails) IsUserRegistered() bool {
	return d.user.Id != (sharedTypes.UUID{})
}

func (d *projectInviteDetails) GetInviteURL(siteURL sharedTypes.URL) *sharedTypes.URL {
	return siteURL.WithPath(fmt.Sprintf(
		"/project/%s/invite/token/%s",
		d.project.Id.String(), d.invite.Token,
	)).WithQuery(url.Values{
		"project_name":    {string(d.project.Name)},
		"user_first_name": {d.sender.DisplayName()},
	})
}

func (d *projectInviteDetails) ValidateForCreation() error {
	if !d.IsUserRegistered() {
		return nil
	}
	if d.sender.Id == d.user.Id {
		return &errors.ValidationError{Msg: "cannot_invite_self"}
	}
	authorizationDetails, err := d.project.GetPrivilegeLevelAuthenticated()
	if err != nil {
		// This user is not a member yet.
		return nil
	}
	if authorizationDetails.IsTokenMember() {
		// We can promote them to a collaborator.
		return nil
	}
	if authorizationDetails.PrivilegeLevel.IsAtLeast(d.invite.PrivilegeLevel) {
		return &errors.ValidationError{Msg: "user is already a project member"}
	}
	return nil
}

func (m *manager) getDetails(ctx context.Context, pi *projectInvite.WithToken) (*projectInviteDetails, error) {
	p := &project.ForProjectInvite{}
	s := &pi.SendingUser
	u := &user.WithPublicInfo{}

	// TODO: merge into single query?
	eg, pCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		if err := m.pm.GetProject(pCtx, pi.ProjectId, p); err != nil {
			return errors.Tag(err, "cannot get project")
		}
		return nil
	})

	eg.Go(func() error {
		if err := m.um.GetUser(pCtx, pi.SendingUser.Id, s); err != nil {
			return errors.Tag(err, "cannot get sender details")
		}
		return nil
	})

	eg.Go(func() error {
		err := m.um.GetUserByEmail(pCtx, pi.Email, u)
		if err == nil {
			// Cleanup any auto-generated first name.
			// The email is normalized to lower case, which reduces the chance
			//  for false-positives `Alice` vs `alice@foo.bar`.
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
		return nil, err
	}

	return &projectInviteDetails{
		project: p,
		invite:  pi,
		sender:  s,
		user:    u,
	}, nil
}
