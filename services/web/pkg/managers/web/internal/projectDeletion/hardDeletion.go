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
	"log"
	"time"

	"github.com/edgedb/edgedb-go"
	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

const (
	parallelHardDeletion = 5
	expireProjectsAfter  = 90 * 24 * time.Hour
)

func (m *manager) HardDeleteExpiredProjects(ctx context.Context, dryRun bool) error {
	eg, pCtx := errgroup.WithContext(ctx)
	// Pass the pCtx to stop fetching ids as soon as any consumer failed.
	queue, errGet := m.dpm.GetExpired(pCtx, expireProjectsAfter)
	if errGet != nil {
		_ = eg.Wait()
		return errGet
	}
	defer func() {
		for range queue {
			// make sure we flush the queue
		}
	}()
	for i := 0; i < parallelHardDeletion; i++ {
		eg.Go(func() error {
			for projectId := range queue {
				if dryRun {
					log.Println(
						"dry-run hard deleting project " + projectId.String(),
					)
					continue
				}
				// Use the original ctx in order to ignore imminent failure
				//  of another consumer.
				if err := m.HardDeleteProject(ctx, projectId); err != nil {
					err = errors.Tag(
						err,
						"hard deletion failed for project "+projectId.String(),
					)
					log.Println(err.Error())
					return err
				}
			}
			return nil
		})
	}
	return eg.Wait()
}

func (m *manager) HardDeleteProject(ctx context.Context, projectId edgedb.UUID) error {
	eg, pCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		if err := m.fm.DeleteProject(pCtx, projectId); err != nil {
			return errors.Tag(err, "cannot destroy files")
		}
		return nil
	})
	// TODO: Consider hard-deleting tracked-changes data (NodeJS did not).
	if err := eg.Wait(); err != nil {
		return err
	}

	// TODO: do extensive cleanup of outbound links, then do cascading delete
	if err := m.dpm.Expire(ctx, projectId); err != nil {
		return errors.Tag(err, "cannot expire deleted project")
	}
	return nil
}
