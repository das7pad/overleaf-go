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
	"encoding/json"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/jwt/projectJWT"
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
	return m.removeMemberFromProject(ctx, projectId, userId)
}

func (m *manager) RemoveMemberFromProject(ctx context.Context, request *types.RemoveProjectMemberRequest) error {
	projectId := request.ProjectId
	userId := request.UserId
	return m.removeMemberFromProject(ctx, projectId, userId)
}

func (m *manager) removeMemberFromProject(ctx context.Context, projectId, userId primitive.ObjectID) error {
	err := mongoTx.For(m.db, ctx, func(ctx context.Context) error {
		if err := m.pm.RemoveMember(ctx, projectId, userId); err != nil {
			return errors.Tag(err, "cannot remove user from project")
		}
		if err := m.tm.RemoveProjectBulk(ctx, userId, projectId); err != nil {
			return errors.Tag(err, "cannot remove project from tags")
		}
		// Clearing the epoch is OK to do at any time.
		// Clear it just ahead of committing the tx.
		err := projectJWT.ClearProjectField(ctx, m.client, projectId)
		if err != nil {
			return err
		}
		return nil
	})
	// Clearing the epoch is OK to do at any time.
	// Clear it immediately after aborting/committing the tx.
	_ = projectJWT.ClearProjectField(ctx, m.client, projectId)
	if err != nil {
		return err
	}
	go m.notifyEditorAboutChanges(projectId, &refreshMembershipDetails{
		Members: true,
	})
	return nil
}

type refreshMembershipDetails struct {
	Invites bool `json:"invites,omitempty"`
	Members bool `json:"members,omitempty"`
}

func (m *manager) notifyEditorAboutChanges(projectId primitive.ObjectID, r *refreshMembershipDetails) {
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()

	payload := []interface{}{r}
	if b, err2 := json.Marshal(payload); err2 == nil {
		_ = m.editorEvents.Publish(ctx, &sharedTypes.EditorEventsMessage{
			RoomId:  projectId,
			Message: "project:membership:changed",
			Payload: b,
		})
	}
}
