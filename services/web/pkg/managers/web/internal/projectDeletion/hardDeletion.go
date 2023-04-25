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

package projectDeletion

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/das7pad/overleaf-go/pkg/constants"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

func (m *manager) HardDeleteExpiredProjects(ctx context.Context, dryRun bool, start time.Time) error {
	nFailed := 0
	err := m.pm.ProcessSoftDeleted(
		ctx,
		start.Add(-constants.ExpireProjectsAfter),
		func(projectId sharedTypes.UUID) bool {
			if dryRun {
				log.Println(
					"dry-run hard deleting project " + projectId.String(),
				)
				return false
			}
			if err := m.HardDeleteProject(ctx, projectId); err != nil {
				err = errors.Tag(
					err,
					"hard deletion failed for project "+projectId.String(),
				)
				nFailed++
				log.Println(err.Error())
			}
			return nFailed == 0
		},
	)
	if err != nil {
		err = errors.Tag(err, "query projects")
	}
	if nFailed != 0 {
		err = errors.Merge(err, errors.New(fmt.Sprintf(
			"archiving failed for %d projects", nFailed,
		)))
	}
	return err
}

func (m *manager) HardDeleteProject(ctx context.Context, projectId sharedTypes.UUID) error {
	if err := m.fm.DeleteProject(ctx, projectId); err != nil {
		return errors.Tag(err, "cannot destroy files")
	}
	if err := m.pm.HardDelete(ctx, projectId); err != nil {
		return errors.Tag(err, "cannot expire deleted project")
	}
	return nil
}
