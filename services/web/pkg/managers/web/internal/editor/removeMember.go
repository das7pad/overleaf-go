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

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/jwt/projectJWT"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/mongoTx"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) LeaveProject(ctx context.Context, request *types.LeaveProjectRequest) error {
	if err := request.Session.CheckIsLoggedIn(); err != nil {
		return err
	}
	projectId := request.ProjectId
	userId := request.Session.User.Id
	for i := 0; i < 10; i++ {
		d, err := m.pm.GetAuthorizationDetails(ctx, projectId, userId, "")
		if err != nil {
			if errors.IsNotAuthorizedError(err) {
				// Already removed.
				return nil
			}
			return errors.Tag(err, "cannot check auth")
		}
		if d.PrivilegeLevel == sharedTypes.PrivilegeLevelOwner {
			return &errors.InvalidStateError{Msg: "cannot leave owned project"}
		}
		err = m.removeMemberFromProject(ctx, projectId, d.Epoch, userId)
		if err != nil {
			if errors.GetCause(err) == project.ErrEpochIsNotStable {
				continue
			}
			return err
		}
		return nil
	}
	return project.ErrEpochIsNotStable
}

func (m *manager) RemoveMemberFromProject(ctx context.Context, request *types.RemoveProjectMemberRequest) error {
	projectId := request.ProjectId
	userId := request.UserId
	epoch := request.Epoch
	return m.removeMemberFromProject(ctx, projectId, epoch, userId)
}

func (m *manager) removeMemberFromProject(ctx context.Context, projectId primitive.ObjectID, epoch int64, userId primitive.ObjectID) error {
	err := mongoTx.For(m.db, ctx, func(ctx context.Context) error {
		if err := m.pm.RemoveMember(ctx, projectId, epoch, userId); err != nil {
			return errors.Tag(err, "cannot remove user from project")
		}
		if err := m.tm.RemoveProjectForUser(ctx, userId, projectId); err != nil {
			return errors.Tag(err, "cannot remove project from tags")
		}
		err := projectJWT.ClearProjectField(ctx, m.client, projectId)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	go m.notifyEditorAboutAccessChanges(projectId, &refreshMembershipDetails{
		Members: true,
	})
	return nil
}
