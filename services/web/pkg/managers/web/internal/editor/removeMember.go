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

package editor

import (
	"context"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) LeaveProject(ctx context.Context, request *types.LeaveProjectRequest) error {
	if err := request.Session.CheckIsLoggedIn(); err != nil {
		return err
	}
	projectId := request.ProjectId
	userId := request.Session.User.Id
	return m.removeMemberFromProject(ctx, projectId, userId, userId)
}

func (m *manager) RemoveMemberFromProject(ctx context.Context, request *types.RemoveProjectMemberRequest) error {
	projectId := request.ProjectId
	memberId := request.MemberId
	return m.removeMemberFromProject(ctx, projectId, request.UserId, memberId)
}

func (m *manager) removeMemberFromProject(ctx context.Context, projectId sharedTypes.UUID, actorId, userId sharedTypes.UUID) error {
	if err := m.pm.RemoveMember(ctx, projectId, actorId, userId); err != nil {
		return errors.Tag(err, "remove user from project")
	}
	go m.notifyEditorAboutAccessChanges(projectId, refreshMembershipDetails{
		Members: true,
		UserId:  userId,
	})
	return nil
}
