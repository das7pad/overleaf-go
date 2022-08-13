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

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/projectInvite"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type projectInviteDetails struct {
	project project.ForProjectInvite
	invite  *projectInvite.WithToken
	sender  user.WithPublicInfo
	user    user.WithPublicInfo
}

func (d *projectInviteDetails) IsUserRegistered() bool {
	return d.user.Id != (sharedTypes.UUID{})
}

func (d *projectInviteDetails) GetInviteURL(siteURL sharedTypes.URL) *sharedTypes.URL {
	return siteURL.WithPath(fmt.Sprintf(
		"/project/%s/invite/token/%s",
		d.invite.ProjectId.String(), d.invite.Token,
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
	alreadyAMember := err == nil
	if !alreadyAMember {
		// This user is not a member yet and the invitation will change that.
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

func (m *manager) getDetails(ctx context.Context, pi *projectInvite.WithToken, actorId sharedTypes.UUID) (*projectInviteDetails, error) {
	d, err := m.pm.GetForProjectInvite(ctx, pi.ProjectId, actorId, pi.Email)
	if err != nil {
		return nil, errors.Tag(err, "get project invite details")
	}
	return &projectInviteDetails{
		project: *d,
		invite:  pi,
		sender:  d.Sender,
		user:    d.User,
	}, nil
}
