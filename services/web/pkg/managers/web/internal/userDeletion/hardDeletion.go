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

package userDeletion

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
	expireUsersAfter     = 90 * 24 * time.Hour
)

func (m *manager) HardDeleteExpiredUsers(ctx context.Context, dryRun bool) error {
	eg, pCtx := errgroup.WithContext(ctx)
	// Pass the pCtx to stop fetching ids as soon as any consumer failed.
	queue, errGet := m.delUM.GetExpired(pCtx, expireUsersAfter)
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
			for userId := range queue {
				if dryRun {
					log.Println("dry-run hard deleting user " + userId.String())
					continue
				}
				// Use the original ctx in order to ignore imminent failure
				//  of another consumer.
				if err := m.HardDeleteUser(ctx, userId); err != nil {
					err = errors.Tag(
						err, "hard deletion failed for user "+userId.String(),
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

func (m *manager) HardDeleteUser(ctx context.Context, userId edgedb.UUID) error {
	eg, pCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		if err := m.cm.DeleteForUser(pCtx, userId); err != nil {
			return errors.Tag(err, "cannot delete tags")
		}
		return nil
	})
	eg.Go(func() error {
		if err := m.nm.DeleteForUser(pCtx, userId); err != nil {
			return errors.Tag(err, "cannot delete notifications")
		}
		return nil
	})
	eg.Go(func() error {
		if err := m.tm.DeleteForUser(pCtx, userId); err != nil {
			return errors.Tag(err, "cannot delete tags")
		}
		return nil
	})
	if err := eg.Wait(); err != nil {
		return err
	}

	if err := m.delUM.Expire(ctx, userId); err != nil {
		return errors.Tag(err, "cannot expire deleted user")
	}
	return nil
}
