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

package userCreation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/oneTimeToken"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/templates"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func randomPassword() (types.UserPassword, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", errors.Tag(err, "cannot generate random password")
	}
	return types.UserPassword(hex.EncodeToString(b)), nil
}

func (m *manager) AdminCreateUser(ctx context.Context, r *types.AdminCreateUserRequest, response *types.AdminCreateUserResponse) error {
	if err := r.Session.CheckIsAdmin(); err != nil {
		return err
	}
	r.Preprocess()
	if err := r.Validate(); err != nil {
		return err
	}

	pw, errPW := randomPassword()
	if errPW != nil {
		return errPW
	}

	u, err := user.NewUser(r.Email)
	if err != nil {
		return err
	}
	u.AuditLog = []user.AuditLogEntry{
		{
			InitiatorId: r.Session.User.Id,
			IpAddress:   r.IPAddress,
			Operation:   user.AuditLogOperationCreateAccount,
			Timestamp:   u.SignUpDate,
		},
	}
	u.OneTimeTokenUse = oneTimeToken.PasswordResetUse
	if err = m.createUser(ctx, u, pw); err != nil {
		if errors.GetCause(err) == user.ErrEmailAlreadyRegistered {
			return user.ErrEmailAlreadyRegistered
		}
		return err
	}

	setPasswordURL := m.options.SiteURL.
		WithPath("/user/activate").
		WithQuery(url.Values{
			"token":   {string(u.OneTimeToken)},
			"user_id": {u.Id.String()},
		})

	response.SetNewPasswordURL = setPasswordURL
	if err := m.sendActivateEmail(ctx, r.Email, setPasswordURL); err != nil {
		return err
	}
	return nil
}

func (m *manager) AdminRegisterUsersPage(_ context.Context, request *types.AdminRegisterUsersPageRequest, response *types.AdminRegisterUsersPageResponse) error {
	if err := request.Session.CheckIsAdmin(); err != nil {
		return err
	}
	response.Data = &templates.AdminRegisterUsersData{
		AngularLayoutData: templates.AngularLayoutData{
			CommonData: templates.CommonData{
				Settings:    m.ps,
				SessionUser: request.Session.User,
				Title:       "Register New Users",
			},
		},
	}
	return nil
}
