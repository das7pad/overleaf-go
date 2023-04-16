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

package types

import (
	"github.com/das7pad/overleaf-go/pkg/asyncForm"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/pkg/templates"
)

func NewCMDCreateUserRequest(email sharedTypes.Email, adminUserId sharedTypes.UUID) CMDCreateUserRequest {
	return CMDCreateUserRequest{
		fromCMD:     true,
		Email:       email,
		InitiatorId: adminUserId,
	}
}

type CMDCreateUserRequest struct {
	fromCMD     bool
	Email       sharedTypes.Email
	InitiatorId sharedTypes.UUID
}

func (r *CMDCreateUserRequest) Preprocess() {
	r.Email = r.Email.Normalize()
}

func (r *CMDCreateUserRequest) Validate() error {
	if !r.fromCMD {
		return errors.New("restricted to command line")
	}
	if err := r.Email.Validate(); err != nil {
		return err
	}
	return nil
}

type CMDCreateUserResponse struct {
	SetNewPasswordURL *sharedTypes.URL
}

type RegisterUserRequest struct {
	WithSession
	IPAddress string `json:"-"`

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

type RegisterUserPageRequest struct {
	WithSession
	templates.SharedProjectData
}

type RegisterUserPageResponse struct {
	Data     *templates.UserRegisterData
	Redirect string
}
