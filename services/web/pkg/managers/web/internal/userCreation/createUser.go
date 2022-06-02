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

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/oneTimeToken"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/login"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) createUser(ctx context.Context, u *user.ForCreation, pw types.UserPassword) error {
	if err := m.um.CheckEmailAlreadyRegistered(ctx, u.Email); err != nil {
		if err == user.ErrEmailAlreadyRegistered {
			// PERF: skip expensive bcrypt hashing.
			return err
		}
		// go the long way and potentially fail again on insert.
	}

	hashedPw, err := login.HashPassword(pw, m.options.BcryptCost)
	if err != nil {
		return err
	}
	u.HashedPassword = hashedPw

	allErrors := &errors.MergedError{}
	for i := 0; i < 10; i++ {
		u.OneTimeToken, err = oneTimeToken.GenerateNewToken()
		if err != nil {
			allErrors.Add(err)
			continue
		}
		if err = m.um.CreateUser(ctx, u); err != nil {
			if err == oneTimeToken.ErrDuplicateOneTimeToken {
				allErrors.Add(err)
				continue
			}
			return errors.Tag(err, "cannot create user")
		}
		return nil
	}
	return allErrors.Finalize()
}
