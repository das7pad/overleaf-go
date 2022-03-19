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

package login

import (
	"context"
	"fmt"
	"log"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/asyncForm"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/session"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) ChangePassword(ctx context.Context, r *types.ChangePasswordRequest, response *types.ChangePasswordResponse) error {
	if err := r.Session.CheckIsLoggedIn(); err != nil {
		return err
	}
	if err := r.Validate(); err != nil {
		return err
	}
	userId := r.Session.User.Id

	u := &user.ForPasswordChange{}
	if err := m.um.GetUser(ctx, userId, u); err != nil {
		return errors.Tag(err, "cannot get user")
	}

	errPW := CheckPassword(&u.HashedPasswordField, r.CurrentPassword)
	if errPW != nil {
		if errors.IsNotAuthorizedError(errPW) {
			return &errors.ValidationError{
				Msg: "Your current password is wrong.",
			}
		}
		return errPW
	}
	err := m.changePassword(
		ctx,
		u,
		r.IPAddress,
		user.AuditLogOperationUpdatePassword,
		r.NewPassword,
	)
	if err != nil {
		return err
	}
	m.postProcessPasswordChange(u, r.Session)
	response.Message = &asyncForm.Message{
		Text: "Password changed",
		Type: "success",
	}
	return nil
}

func (m *manager) changePassword(ctx context.Context, u *user.ForPasswordChange, ip, action string, password types.UserPassword) error {
	if err := password.CheckForEmailMatch(u.Email); err != nil {
		return err
	}
	hashedPassword, err := HashPassword(password, m.options.BcryptCost)
	if err != nil {
		return err
	}
	errChange := m.um.ChangePassword(ctx, u, ip, action, hashedPassword)
	if errChange != nil {
		return errors.Tag(errChange, "cannot change password")
	}
	return nil
}

func (m *manager) postProcessPasswordChange(u *user.ForPasswordChange, s *session.Session) {
	// We need to clear sessions and email the user. Ignore any request aborts.
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	eg := errgroup.Group{}
	eg.Go(func() error {
		if s == nil {
			return m.sm.DestroyAllForUser(ctx, u.Id)
		}
		otherSessions, err := s.GetOthers(ctx)
		if err != nil {
			return errors.Tag(err, "cannot get other sessions")
		}
		if len(otherSessions.Sessions) == 0 {
			return nil
		}
		if err = s.DestroyOthers(ctx, otherSessions); err != nil {
			return errors.Tag(err, "cannot destroy other sessions")
		}
		return nil
	})
	eg.Go(func() error {
		err := m.emailSecurityAlert(
			ctx, &u.WithPublicInfo, "password changed",
			fmt.Sprintf(
				"your password has been changed on your account %s",
				u.Email,
			),
		)
		if err != nil {
			return errors.Tag(err, "cannot notify user")
		}
		return nil
	})
	if err := eg.Wait(); err != nil {
		log.Printf(
			"%s: cannot finalize password change: %s",
			u.Id.String(), err.Error(),
		)
	}
}
