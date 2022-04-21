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
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/login"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) createUser(ctx context.Context, emailAddress sharedTypes.Email, pw types.UserPassword, ip string) (*user.ForCreation, error) {
	if m.um.GetUserByEmail(ctx, emailAddress, &user.IdField{}) == nil {
		// PERF: skip expensive bcrypt hashing.
		return nil, user.ErrEmailAlreadyRegistered
	}

	hashedPw, err := login.HashPassword(pw, m.options.BcryptCost)
	if err != nil {
		return nil, err
	}
	u := user.NewUser(emailAddress, hashedPw)
	if ip != "" {
		u.LastLoginIp = ip
	}
	if err = m.um.CreateUser(ctx, u); err != nil {
		return nil, errors.Tag(err, "cannot create user")
	}
	return u, nil
}
