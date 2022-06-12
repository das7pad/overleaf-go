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

	"github.com/das7pad/overleaf-go/pkg/errors"
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

	err := m.pim.Accept(ctx, projectId, userId, request.Token)
	if err != nil {
		return errors.Tag(err, "cannot get invite")
	}

	go m.notifyEditorAboutChanges(projectId, &refreshMembershipDetails{
		Invites: true,
		Members: true,
	})

	response.RedirectTo = "/project/" + projectId.String()
	return nil
}
