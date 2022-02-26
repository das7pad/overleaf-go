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
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/mongoTx"
	"github.com/das7pad/overleaf-go/pkg/session"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/login"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

const parallelDeletion = 5

func (m *manager) DeleteUser(ctx context.Context, request *types.DeleteUserRequest) error {
	if err := request.Session.CheckIsLoggedIn(); err != nil {
		return err
	}
	userId := request.Session.User.Id
	ipAddress := request.IPAddress

	u := &user.ForDeletion{}
	if err := m.um.GetUser(ctx, userId, u); err != nil {
		if errors.IsNotFoundError(err) {
			m.destroySessionInBackground(request.Session)
		}
		return errors.Tag(err, "cannot get user")
	}
	errPW := login.CheckPassword(&u.HashedPasswordField, request.Password)
	if errPW != nil {
		return errPW
	}

	err := mongoTx.For(m.db, ctx, func(sCtx context.Context) error {
		projects, errGetProjects := m.pm.ListProjects(ctx, userId)
		if errGetProjects != nil {
			return errors.Tag(errGetProjects, "cannot get projects")
		}

		queue := make(chan *project.ListViewPrivate, parallelDeletion)
		eg, pCtx := errgroup.WithContext(sCtx)
		go func() {
			<-pCtx.Done()
			if pCtx.Err() != nil {
				for range queue {
					// Clear the queue
				}
			}
		}()

		eg.Go(func() error {
			defer close(queue)
			for _, p := range projects {
				queue <- p
			}
			return nil
		})
		for i := 0; i < parallelDeletion; i++ {
			eg.Go(func() error {
				for p := range queue {
					if p.OwnerRef == userId {
						err := m.pDelM.DeleteProjectInTx(
							ctx, pCtx, &types.DeleteProjectRequest{
								Session:   request.Session,
								ProjectId: p.Id,
								IPAddress: request.IPAddress,
								EpochHint: &p.Epoch,
							},
						)
						if err != nil {
							return errors.Tag(
								err, "cannot delete project "+p.Id.String(),
							)
						}
					} else {
						err := m.pm.RemoveMember(pCtx, p.Id, p.Epoch, userId)
						if err != nil {
							return errors.Tag(
								err,
								"cannot remove user from project "+p.Id.String(),
							)
						}
					}
				}
				return nil
			})
		}

		eg.Go(func() error {
			otherSessions, err := request.Session.GetOthers(pCtx)
			if err != nil {
				return errors.Tag(err, "cannot get other sessions")
			}
			if len(otherSessions.Sessions) == 0 {
				return nil
			}
			err = request.Session.DestroyOthers(pCtx, otherSessions)
			if err != nil {
				return errors.Tag(err, "cannot clear other sessions")
			}
			return nil
		})

		eg.Go(func() error {
			if err := m.delUM.Create(pCtx, u, userId, ipAddress); err != nil {
				return errors.Tag(err, "cannot create deleted user")
			}
			return nil
		})
		if err := eg.Wait(); err != nil {
			return err
		}

		if err := m.um.Delete(sCtx, userId, u.Epoch); err != nil {
			return errors.Tag(err, "cannot delete user")
		}
		return nil
	})

	if err != nil {
		return err
	}

	// The user has been deleted by now.
	// Run cleanup in the background as they cannot retry.
	m.destroySessionInBackground(request.Session)
	return nil
}

func (m *manager) destroySessionInBackground(s *session.Session) {
	bCtx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	_ = s.Destroy(bCtx)
}
