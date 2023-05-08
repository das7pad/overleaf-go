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
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/session"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/login"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) DeleteUser(ctx context.Context, request *types.DeleteUserRequest) error {
	if err := request.Session.CheckIsLoggedIn(); err != nil {
		return err
	}
	userId := request.Session.User.Id
	ipAddress := request.IPAddress

	u := user.HashedPasswordField{}
	if err := m.um.GetUser(ctx, userId, &u); err != nil {
		if errors.IsNotFoundError(err) {
			m.destroySessionInBackground(request.Session)
		}
		return errors.Tag(err, "get user")
	}
	if err := login.CheckPassword(u, request.Password); err != nil {
		return err
	}

	eg, pCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		err := m.pDelM.DeleteUsersOwnedProjects(pCtx, userId, ipAddress)
		if err != nil {
			return errors.Tag(err, "delete users projects")
		}
		return nil
	})

	eg.Go(func() error {
		otherSessions, err := request.Session.GetOthers(pCtx)
		if err != nil {
			return errors.Tag(err, "get other sessions")
		}
		if len(otherSessions.Sessions) == 0 {
			return nil
		}
		err = request.Session.DestroyOthers(pCtx, otherSessions)
		if err != nil {
			return errors.Tag(err, "clear other sessions")
		}
		return nil
	})
	if err := eg.Wait(); err != nil {
		return err
	}

	if err := m.um.SoftDelete(ctx, userId, ipAddress); err != nil {
		return errors.Tag(err, "delete user")
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
