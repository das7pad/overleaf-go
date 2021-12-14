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

package userCreation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/oneTimeToken"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/mongoTx"
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

	var u *user.ForCreation
	var t oneTimeToken.OneTimeToken
	err := mongoTx.For(m.db, ctx, func(sCtx context.Context) error {
		var err error
		u, err = m.createUser(sCtx, r.Email, pw, "")
		if err != nil {
			return err
		}
		t, err = m.oTTm.NewForPasswordSet(sCtx, u.Id, r.Email)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if errors.GetCause(err) == user.ErrEmailAlreadyRegistered {
			return user.ErrEmailAlreadyRegistered
		}
		return err
	}

	setPasswordURL := m.options.SiteURL.
		WithPath("/user/activate").
		WithQuery(url.Values{
			"token":   {string(t)},
			"user_id": {u.Id.Hex()},
		})

	response.SetNewPasswordURL = setPasswordURL
	if err = m.sendActivateEmail(ctx, r.Email, setPasswordURL); err != nil {
		return err
	}
	return nil
}
