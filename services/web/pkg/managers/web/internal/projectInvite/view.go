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
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/projectInvite"
	"github.com/das7pad/overleaf-go/services/web/pkg/templates"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) ViewProjectInvite(ctx context.Context, r *types.ViewProjectInvitePageRequest, response *types.ViewProjectInvitePageResponse) error {
	if err := r.Session.CheckIsLoggedIn(); err != nil {
		return err
	}
	r.SharedProjectData.Preprocess()
	if err := r.Validate(); err != nil {
		return err
	}
	projectId := r.ProjectId
	userId := r.Session.User.Id

	valid := true
	{
		d, err := m.pm.GetAuthorizationDetails(ctx, projectId, userId, "")
		if err == nil {
			// The user may have redeemed the invitation already.
			// Allow them to click on the link in the email again.
			if d.AccessSource == project.AccessSourceInvite ||
				// They might have been promoted to project owner.
				d.AccessSource == project.AccessSourceOwner {
				response.Redirect = "/project/" + projectId.Hex()
				return nil
			}
		} else if errors.IsNotAuthorizedError(err) {
			// The invitation might change this :)
		} else if errors.IsNotFoundError(err) {
			// The project has been deleted. Invitations are not deleted
			//  when soft/hard deleting a project as they expire after 30days,
			//  which is less than the 90days soft-deletion delay.
			valid = false
		} else {
			return err
		}
	}
	{
		pi := &projectInvite.IdField{}
		if err := m.pim.GetByToken(ctx, projectId, r.Token, pi); err == nil {
			// Happy path
		} else if errors.IsNotFoundError(err) {
			valid = false
		} else {
			return errors.Tag(err, "cannot get invite")
		}
	}

	title := "Project Invite"
	if !valid {
		title = "Invalid Invite"
	}
	response.Data = &templates.ProjectViewInviteData{
		MarketingLayoutData: templates.MarketingLayoutData{
			JsLayoutData: templates.JsLayoutData{
				CommonData: templates.CommonData{
					Settings:    m.ps,
					SessionUser: r.Session.User,
					Title:       title,
				},
			},
		},
		SharedProjectData: r.SharedProjectData,
		ProjectId:         projectId,
		Token:             r.Token,
		Valid:             valid,
	}
	return nil
}
