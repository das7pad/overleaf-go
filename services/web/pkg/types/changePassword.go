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
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/asyncForm"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/oneTimeToken"
	"github.com/das7pad/overleaf-go/pkg/session"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/pkg/templates"
)

type ChangePasswordRequest struct {
	Session   *session.Session `json:"-"`
	IPAddress string           `json:"-"`

	CurrentPassword    UserPassword `json:"currentPassword"`
	NewPassword        UserPassword `json:"newPassword1"`
	NewPasswordConfirm UserPassword `json:"newPassword2"`
}

func (r *ChangePasswordRequest) Validate() error {
	if err := r.NewPassword.Validate(); err != nil {
		return errors.Tag(err, "new password")
	}
	if r.CurrentPassword == r.NewPassword {
		return &errors.ValidationError{
			Msg: "current and new password must differ",
		}
	}
	if r.NewPassword != r.NewPasswordConfirm {
		return &errors.ValidationError{Msg: "new passwords do not match"}
	}
	return nil
}

type ChangePasswordResponse = asyncForm.Response

type SetPasswordRequest struct {
	Session   *session.Session `json:"-"`
	IPAddress string           `json:"-"`

	Token    oneTimeToken.OneTimeToken `json:"passwordResetToken"`
	Password UserPassword              `json:"password"`
}

type SetPasswordResponse = asyncForm.Response

type SetPasswordPageRequest struct {
	Session *session.Session          `form:"-"`
	Email   sharedTypes.Email         `form:"email"`
	Token   oneTimeToken.OneTimeToken `form:"passwordResetToken"`
}

type SetPasswordPageResponse struct {
	Data     *templates.UserSetPasswordData
	Redirect string
}

type RequestPasswordResetRequest struct {
	Email sharedTypes.Email `json:"email"`
}

func (r *RequestPasswordResetRequest) Preprocess() {
	r.Email = r.Email.Normalize()
}

func (r *RequestPasswordResetRequest) Validate() error {
	if err := r.Email.Validate(); err != nil {
		return err
	}
	return nil
}

type RequestPasswordResetPageRequest struct {
	Session *session.Session  `form:"-"`
	Email   sharedTypes.Email `form:"email"`
}

type RequestPasswordResetPageResponse struct {
	Data     *templates.UserPasswordResetData
	Redirect string
}

type ActivateUserPageRequest struct {
	Session   *session.Session          `form:"-"`
	UserIdHex string                    `form:"user_id"`
	Token     oneTimeToken.OneTimeToken `form:"token"`
}

func (r *ActivateUserPageRequest) Validate() error {
	if !primitive.IsValidObjectID(r.UserIdHex) {
		return &errors.ValidationError{Msg: "missing user_id"}
	}
	if err := r.Token.Validate(); err != nil {
		return err
	}
	return nil
}

type ActivateUserPageResponse struct {
	Data     *templates.UserActivateData
	Redirect string
}
