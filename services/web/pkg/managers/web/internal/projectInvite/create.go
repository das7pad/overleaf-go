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
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/projectInvite"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) CreateProjectInvite(ctx context.Context, request *types.CreateProjectInviteRequest) error {
	request.Preprocess()
	if err := request.Validate(); err != nil {
		return err
	}

	now := time.Now()
	pi := &projectInvite.WithToken{}
	pi.Email = request.Email
	pi.Expires = now.Add(30 * 24 * time.Hour)
	pi.PrivilegeLevel = request.PrivilegeLevel
	pi.ProjectId = request.ProjectId
	pi.SendingUser.Id = request.SenderUserId

	d, err := m.getDetails(ctx, pi, request.SenderUserId)
	if err != nil {
		return err
	}
	if err = d.ValidateForCreation(); err != nil {
		return err
	}

	if err = m.pim.Create(ctx, pi); err != nil {
		return errors.Tag(err, "cannot create invite")
	}

	// Possible false negative error, request refresh.
	defer m.notifyEditorAboutChanges(
		request.ProjectId, &refreshMembershipDetails{Invites: true},
	)

	if err = m.sendEmail(ctx, d); err != nil {
		return err
	}
	return nil
}
