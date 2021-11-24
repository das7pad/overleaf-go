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
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/session"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type LogoutRequest struct {
	Session *session.Session `json:"-"`
}

type LogoutResponse = asyncForm.Response

type LoginRequest struct {
	Session   *session.Session  `json:"-"`
	IPAddress string            `json:"-"`
	Email     sharedTypes.Email `json:"email"`
	Password  string            `json:"password"`
}

func (r *LoginRequest) Preprocess() {
	r.Email = r.Email.Normalize()
}

func (r *LoginRequest) Validate() error {
	if len(r.Password) < 6 {
		return &errors.ValidationError{Msg: "password too short, min 6"}
	}
	if len(r.Password) > 72 {
		return &errors.ValidationError{Msg: "password too long, max 72"}
	}
	if err := r.Email.Validate(); err != nil {
		return err
	}
	return nil
}

type LoginResponse = asyncForm.Response

type GetLoggedInUserJWTRequest struct {
	Session *session.Session `json:"-"`
}

type GetLoggedInUserJWTResponse string

type ClearSessionsRequest struct {
	Session   *session.Session `json:"-"`
	IPAddress string           `json:"-"`
}
