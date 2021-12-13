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

package login

import (
	"context"
	"fmt"
	"net/url"

	"github.com/das7pad/overleaf-go/pkg/email"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/mongoTx"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) SetPassword(ctx context.Context, r *types.SetPasswordRequest) error {
	if err := r.Validate(); err != nil {
		return err
	}
	u := &user.ForPasswordChange{}

	// NOTE: Use a tx for not expiring the token for real until we action the
	//        actual password change.
	err := mongoTx.For(m.db, ctx, func(sCtx context.Context) error {
		d, errResolve := m.oTTm.ResolveAndExpirePasswordResetToken(
			sCtx, r.Token,
		)
		if errResolve != nil {
			return errors.Tag(errResolve, "cannot get token data")
		}
		if err := m.um.GetUserByEmail(sCtx, d.Email, u); err != nil {
			return errors.Tag(err, "cannot get user")
		}
		if u.Id.Hex() != d.HexUserId {
			return &errors.UnprocessableEntityError{
				Msg: "owner of email changed",
			}
		}
		{
			errSamePW := CheckPassword(&u.HashedPasswordField, r.Password)
			if errSamePW == nil {
				return &errors.ValidationError{
					Msg: "cannot re-use same password",
				}
			}
			if !errors.IsNotAuthorizedError(errSamePW) {
				return errSamePW
			}
		}
		return m.changePassword(
			sCtx,
			u,
			r.IPAddress,
			user.AuditLogOperationResetPassword,
			r.Password,
		)
	})
	if err != nil {
		return err
	}
	m.postProcessPasswordChange(u, nil)
	return nil
}

func (m *manager) RequestPasswordReset(ctx context.Context, r *types.RequestPasswordResetRequest) error {
	r.Preprocess()
	if err := r.Validate(); err != nil {
		return err
	}
	u := &user.WithPublicInfo{}
	if err := m.um.GetUserByEmail(ctx, r.Email, u); err != nil {
		if errors.IsNotFoundError(err) {
			return &errors.ValidationError{
				Msg: "That email address is not registered, sorry.",
			}
		}
		return errors.Tag(err, "cannot get user")
	}
	token, err := m.oTTm.NewForPasswordReset(ctx, u.Id, r.Email)
	if err != nil {
		return errors.Tag(err, "cannot create one time token")
	}

	e := &email.Email{
		Content: &email.CTAContent{
			PublicOptions: m.emailOptions.Public,
			Message: email.Message{
				fmt.Sprintf(
					"We got a request to reset your %s password.",
					m.options.AppName,
				),
			},
			SecondaryMessage: email.Message{
				"If you ignore this message, your password won't be changed.",
				"If you didn't request a password reset, let us know.",
			},
			Title:   "Password Reset",
			CTAText: "Reset password",
			CTAURL: m.options.SiteURL.
				WithPath("/user/password/set").
				WithQuery(url.Values{
					"passwordResetToken": {string(token)},
					"email":              {string(u.Email)},
				}),
		},
		Subject: "Password Reset - " + m.options.AppName,
		To: &email.Identity{
			Address:     u.Email,
			DisplayName: u.DisplayName(),
		},
	}
	if err = e.Send(ctx, m.emailOptions.Send); err != nil {
		return errors.Tag(err, "cannot email password reset token")
	}
	return nil
}
