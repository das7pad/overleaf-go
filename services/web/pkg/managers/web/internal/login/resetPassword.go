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

package login

import (
	"context"
	"fmt"
	"net/url"

	"github.com/das7pad/overleaf-go/pkg/email"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/pkg/templates"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) SetPassword(ctx context.Context, r *types.SetPasswordRequest, res *types.SetPasswordResponse) error {
	if err := r.Token.Validate(); err != nil {
		return err
	}
	if err := r.Password.Validate(); err != nil {
		res.SetCustomFormMessage("invalid-password", err)
		return err
	}
	u := user.ForPasswordChange{}
	if err := m.um.GetByPasswordResetToken(ctx, r.Token, &u); err != nil {
		if errors.IsNotFoundError(err) {
			res.SetCustomFormMessage("token-expired", err)
		}
		return errors.Tag(err, "get token data")
	}
	if err := r.Password.CheckForEmailMatch(u.Email); err != nil {
		res.SetCustomFormMessage("invalid-password", err)
		return err
	}
	{
		errSamePW := CheckPassword(u.HashedPasswordField, r.Password)
		if errSamePW == nil {
			return &errors.ValidationError{Msg: "password re-use not allowed"}
		}
		if !errors.IsNotAuthorizedError(errSamePW) {
			return errSamePW
		}
	}
	err := m.changePassword(
		ctx,
		u,
		r.IPAddress,
		user.AuditLogOperationResetPassword,
		r.Password,
	)
	if err != nil {
		return err
	}
	m.postProcessPasswordChange(u, nil)
	if r.Session.PasswordResetToken != "" {
		r.Session.PasswordResetToken = ""
		_, _ = r.Session.Save(ctx)
	}
	return nil
}

func (m *manager) SetPasswordPage(_ context.Context, request *types.SetPasswordPageRequest, response *types.SetPasswordPageResponse) error {
	var e sharedTypes.Email
	var q url.Values
	if request.Email.Validate() == nil {
		// .Email is an optional hint.
		e = request.Email
		q = url.Values{"email": {string(e)}}
	}
	if request.Token == "" && request.Session.PasswordResetToken == "" {
		response.Redirect = m.siteURL.
			WithPath("/user/password/reset").
			WithQuery(q).
			String()
	}
	if request.Token != "" {
		if err := request.Token.Validate(); err != nil {
			return err
		}
		request.Session.PasswordResetToken = request.Token
		response.Redirect = m.siteURL.
			WithPath("/user/password/set").
			WithQuery(q).
			String()
		return nil
	}

	response.Data = &templates.UserSetPasswordData{
		MarketingLayoutData: templates.MarketingLayoutData{
			CommonData: templates.CommonData{
				Settings:    m.ps,
				Session:     request.Session.PublicData,
				TitleLocale: "set_password",
				Viewport:    true,
			},
		},
		Email:              e,
		PasswordResetToken: request.Session.PasswordResetToken,
	}
	return nil
}

func (m *manager) RequestPasswordReset(ctx context.Context, r *types.RequestPasswordResetRequest) error {
	r.Preprocess()
	if err := r.Validate(); err != nil {
		return err
	}
	u := user.WithPublicInfo{}
	if err := m.um.GetUserByEmail(ctx, r.Email, &u); err != nil {
		if errors.IsNotFoundError(err) {
			return &errors.ValidationError{
				Msg: "That email address is not registered, sorry.",
			}
		}
		return errors.Tag(err, "get user")
	}
	token, err := m.oTTm.NewForPasswordReset(ctx, u.Id, r.Email)
	if err != nil {
		return errors.Tag(err, "create one time token")
	}

	e := email.Email{
		Content: &email.CTAContent{
			PublicOptions: m.emailOptions.Public,
			Message: email.Message{
				fmt.Sprintf(
					"We got a request to reset your %s password.",
					m.appName,
				),
			},
			SecondaryMessage: email.Message{
				"If you ignore this message, your password won't be changed.",
				"If you didn't request a password reset, let us know.",
			},
			Title:   "Password Reset",
			CTAText: "Reset password",
			CTAURL: m.siteURL.
				WithPath("/user/password/set").
				WithQuery(url.Values{
					"passwordResetToken": {string(token)},
					"email":              {string(u.Email)},
				}),
		},
		Subject: "Password Reset - " + m.appName,
		To: email.Identity{
			Address:     u.Email,
			DisplayName: u.DisplayName(),
		},
	}
	if err = e.Send(ctx, m.emailOptions.Send); err != nil {
		return errors.Tag(err, "email password reset token")
	}
	return nil
}

func (m *manager) RequestPasswordResetPage(_ context.Context, request *types.RequestPasswordResetPageRequest, response *types.RequestPasswordResetPageResponse) error {
	var e sharedTypes.Email
	if request.Email.Validate() == nil {
		// Prefilling the form is optional.
		e = request.Email
	}

	response.Data = &templates.UserPasswordResetData{
		MarketingLayoutData: templates.MarketingLayoutData{
			CommonData: templates.CommonData{
				Settings:    m.ps,
				Session:     request.Session.PublicData,
				TitleLocale: "reset_password",
				Viewport:    true,
			},
		},
		Email: e,
	}
	return nil
}
