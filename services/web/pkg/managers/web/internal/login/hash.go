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

package login

import (
	"golang.org/x/crypto/bcrypt"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func CheckPassword(u user.HashedPasswordField, password types.UserPassword) error {
	if err := password.Validate(); err != nil {
		return err
	}
	err := bcrypt.CompareHashAndPassword(
		[]byte(u.HashedPassword),
		[]byte(password),
	)
	if err != nil {
		if err == bcrypt.ErrMismatchedHashAndPassword {
			return &errors.NotAuthorizedError{}
		}
		return errors.Tag(err, "cannot check user credentials")
	}
	return nil
}

func HashPassword(password types.UserPassword, cost int) (string, error) {
	if err := password.Validate(); err != nil {
		return "", err
	}

	hashed, err := bcrypt.GenerateFromPassword(
		[]byte(password), cost,
	)
	if err != nil {
		return "", errors.Tag(err, "cannot hash password")
	}
	return string(hashed), nil
}
