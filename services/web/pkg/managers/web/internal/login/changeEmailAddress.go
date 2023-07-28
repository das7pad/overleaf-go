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

package login

import (
	"context"
	"fmt"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) ChangeEmailAddress(ctx context.Context, r *types.ChangeEmailAddressRequest) error {
	if err := r.Session.CheckIsLoggedIn(); err != nil {
		return err
	}
	r.Preprocess()
	if err := r.Validate(); err != nil {
		return err
	}
	userId := r.Session.User.Id

	u := user.ForEmailChange{}
	if err := m.um.GetUser(ctx, userId, &u); err != nil {
		return errors.Tag(err, "get user")
	}
	oldEmail := u.Email
	newEmail := r.Email
	if oldEmail == newEmail {
		// Already changed.
		r.Session.User.Email = newEmail
		return nil
	}

	{
		err := m.um.ChangeEmailAddress(ctx, u, r.IPAddress, newEmail)
		if err != nil {
			return errors.Tag(err, "change email")
		}
	}
	r.Session.User.Email = newEmail

	action := "change of primary email address"
	actionDescribed := fmt.Sprintf(
		"the primary email address on your account was changed to %s",
		newEmail,
	)

	mergedErr := errors.MergedError{}
	{
		uOld := r.Session.User.ToPublicUserInfo()
		uOld.Email = oldEmail
		err := m.emailSecurityAlert(ctx, uOld, action, actionDescribed)
		if err != nil {
			mergedErr.Add(errors.Tag(err, "notify old email"))
		}
	}
	{
		uNew := r.Session.User.ToPublicUserInfo()
		uNew.Email = newEmail
		err := m.emailSecurityAlert(ctx, uNew, action, actionDescribed)
		if err != nil {
			mergedErr.Add(errors.Tag(err, "notify new email"))
		}
	}
	return mergedErr.Finalize()
}
