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
	"github.com/das7pad/overleaf-go/pkg/mongoTx"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) RevokeProjectInvite(ctx context.Context, request *types.RevokeProjectInviteRequest) error {
	projectId := request.ProjectId
	inviteId := request.InviteId

	err := mongoTx.For(m.db, ctx, func(ctx context.Context) error {
		if err := m.pm.BumpEpoch(ctx, projectId); err != nil {
			return errors.Tag(err, "cannot bump epoch")
		}
		if err := m.pim.Delete(ctx, projectId, inviteId); err != nil {
			return errors.Tag(err, "cannot delete invite")
		}
		if err := m.nm.RemoveByKeyOnly(ctx, getKey(inviteId)); err != nil {
			return errors.Tag(err, "cannot delete invite notification")
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

	go m.notifyEditorAboutChanges(projectId, &refreshMembershipDetails{
		Invites: true,
	})
	return nil
}
