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
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) SetMemberPrivilegeLevelInProject(ctx context.Context, r *types.SetMemberPrivilegeLevelInProjectRequest) error {
	if err := r.PrivilegeLevel.Validate(); err != nil {
		return err
	}
	err := m.pm.GrantMemberAccess(
		ctx, r.ProjectId, r.UserId, r.MemberId, r.PrivilegeLevel,
	)
	if err != nil {
		return errors.Tag(err, "update project member")
	}

	go m.notifyEditorAboutAccessChanges(r.ProjectId, refreshMembershipDetails{
		Members: true,
		UserId:  r.MemberId,
	})
	return nil
}
