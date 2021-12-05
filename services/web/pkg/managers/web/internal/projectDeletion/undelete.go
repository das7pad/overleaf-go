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

package projectDeletion

import (
	"context"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/deletedProject"
	"github.com/das7pad/overleaf-go/pkg/mongoTx"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) UnDeleteProject(ctx context.Context, request *types.UnDeleteProjectRequest) error {
	if err := request.Session.CheckIsLoggedIn(); err != nil {
		return err
	}
	userId := request.Session.User.Id
	projectId := request.ProjectId

	return mongoTx.For(m.db, ctx, func(sCtx context.Context) error {
		dp := &deletedProject.Full{}
		if err := m.dpm.Get(sCtx, projectId, dp); err != nil {
			return errors.Tag(err, "cannot get deleted project")
		}
		if dp.DeleterData.DeletedProjectOwnerId != userId {
			return &errors.NotAuthorizedError{}
		}
		if dp.Project == nil {
			return &errors.InvalidStateError{Msg: "already hard-deleted"}
		}

		names, err := m.pm.GetProjectNames(sCtx, userId)
		if err != nil {
			return errors.Tag(err, "cannot get other project names")
		}

		p := dp.Project
		p.Name = names.MakeUnique(p.Name)

		if err = m.pm.Restore(sCtx, p); err != nil {
			return errors.Tag(err, "cannot restore project")
		}

		if err = m.dpm.Delete(sCtx, projectId); err != nil {
			return errors.Tag(err, "cannot delete deleted project")
		}
		return nil
	})
}
