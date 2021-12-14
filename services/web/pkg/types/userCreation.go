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

package types

import (
	"github.com/das7pad/overleaf-go/pkg/asyncForm"
	"github.com/das7pad/overleaf-go/pkg/session"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type AdminCreateUserRequest struct {
	Session *session.Session `json:"-"`

	Email sharedTypes.Email `json:"email"`
}

func (r *AdminCreateUserRequest) Preprocess() {
	r.Email = r.Email.Normalize()
}

func (r *AdminCreateUserRequest) Validate() error {
	if err := r.Email.Validate(); err != nil {
		return err
	}
	return nil
}

type AdminCreateUserResponse struct {
	SetNewPasswordURL *sharedTypes.URL `json:"setNewPasswordUrl"`
}

type RegisterUserRequest struct {
	Session   *session.Session `json:"-"`
	IPAddress string           `json:"-"`

	Email    sharedTypes.Email `json:"email"`
	Password UserPassword      `json:"password"`
}

func (r *RegisterUserRequest) Preprocess() {
	r.Email = r.Email.Normalize()
}

func (r *RegisterUserRequest) Validate() error {
	if err := r.Email.Validate(); err != nil {
		return err
	}
	if err := r.Password.Validate(); err != nil {
		return err
	}
	if err := r.Password.CheckForEmailMatch(r.Email); err != nil {
		return err
	}
	return nil
}

type RegisterUserResponse = asyncForm.Response
