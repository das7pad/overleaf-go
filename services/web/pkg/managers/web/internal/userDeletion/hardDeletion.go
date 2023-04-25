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

package userDeletion

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/das7pad/overleaf-go/pkg/constants"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

func (m *manager) HardDeleteExpiredUsers(ctx context.Context, dryRun bool, start time.Time) error {
	nFailed := 0
	err := m.um.ProcessSoftDeleted(
		ctx,
		start.Add(-constants.ExpireUsersAfter),
		func(userId sharedTypes.UUID) bool {
			if dryRun {
				log.Println("dry-run hard deleting user " + userId.String())
				return false
			}
			if err := m.um.HardDelete(ctx, userId); err != nil {
				err = errors.Tag(
					err, "hard deletion failed for user "+userId.String(),
				)
				log.Println(err.Error())
				nFailed++
				return false
			}
			return nFailed == 0
		},
	)
	if err != nil {
		err = errors.Tag(err, "query users")
	}
	if nFailed != 0 {
		err = errors.Merge(err, errors.New(fmt.Sprintf(
			"archiving failed for %d users", nFailed,
		)))
	}
	return err
}
