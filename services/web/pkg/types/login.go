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
	"strings"

	"github.com/das7pad/overleaf-go/pkg/asyncForm"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/session"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/templates"
)

type LogoutRequest struct {
	Session *session.Session `json:"-"`
}

type LogoutResponse = asyncForm.Response

type LogoutPageRequest struct {
	Session *session.Session `form:"-"`
}

type LogoutPageResponse struct {
	Data *templates.UserLogoutData
}

type UserPassword string

func (p UserPassword) Validate() error {
	if len(p) < 6 {
		return &errors.ValidationError{Msg: "password too short, min 6"}
	}
	if len(p) > 72 {
		return &errors.ValidationError{Msg: "password too long, max 72"}
	}
	return nil
}

func (p UserPassword) CheckForEmailMatch(email sharedTypes.Email) error {
	if strings.Contains(strings.ToLower(string(p)), email.LocalPart()) {
		return &errors.ValidationError{Msg: "password contain email part"}
	}
	return nil
}

type LoginRequest struct {
	Session   *session.Session  `json:"-"`
	IPAddress string            `json:"-"`
	Email     sharedTypes.Email `json:"email"`
	Password  UserPassword      `json:"password"`
}

func (r *LoginRequest) Preprocess() {
	r.Email = r.Email.Normalize()
}

func (r *LoginRequest) Validate() error {
	if err := r.Password.Validate(); err != nil {
		return err
	}
	if err := r.Email.Validate(); err != nil {
		return err
	}
	return nil
}

type LoginResponse = asyncForm.Response

type LoginPageRequest struct {
	Session *session.Session `form:"-"`
}

type LoginPageResponse struct {
	Data     *templates.UserLoginData
	Redirect string
}

type GetLoggedInUserJWTRequest struct {
	Session *session.Session `json:"-"`
}

type GetLoggedInUserJWTResponse string

type ClearSessionsRequest struct {
	Session   *session.Session `json:"-"`
	IPAddress string           `json:"-"`
}

type SessionsPageRequest struct {
	Session *session.Session `form:"-"`
}

type SessionsPageResponse struct {
	Data *templates.UserSessionsData
}

type ChangeEmailAddressRequest struct {
	Session   *session.Session `json:"-"`
	IPAddress string           `json:"-"`

	Email sharedTypes.Email `json:"email"`
}

func (r *ChangeEmailAddressRequest) Preprocess() {
	r.Email = r.Email.Normalize()
}

func (r *ChangeEmailAddressRequest) Validate() error {
	if err := r.Email.Validate(); err != nil {
		return err
	}
	return nil
}

type SetUserName struct {
	Session *session.Session `json:"-"`

	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type SettingsPageRequest struct {
	Session *session.Session `form:"-"`
}

type SettingsPageResponse struct {
	Data *templates.UserSettingsData
}
