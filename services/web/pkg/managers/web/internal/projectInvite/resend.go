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
	"github.com/das7pad/overleaf-go/pkg/models/projectInvite"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) ResendProjectInvite(ctx context.Context, request *types.ResendProjectInviteRequest) error {
	projectId := request.ProjectId
	inviteId := request.InviteId

	pi := &projectInvite.WithToken{}
	if err := m.pim.GetById(ctx, projectId, inviteId, pi); err != nil {
		return errors.Tag(err, "cannot get invite")
	}

	d, err := m.getDetails(ctx, pi)
	if err != nil {
		return err
	}

	if err = m.resendNotification(ctx, d); err != nil {
		return errors.Tag(err, "cannot create notification")
	}

	if err = m.sendEmail(ctx, d); err != nil {
		return err
	}
	return nil
}
