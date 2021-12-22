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
	"github.com/das7pad/overleaf-go/services/web/pkg/templates"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) ConfirmEmail(ctx context.Context, r *types.ConfirmEmailRequest) error {
	if err := r.Validate(); err != nil {
		return err
	}

	// NOTE: Use a tx for not expiring the token for real until we action the
	//        actual email confirmation.
	return mongoTx.For(m.db, ctx, func(sCtx context.Context) error {
		d, err := m.oTTm.ResolveAndExpireEmailConfirmationToken(
			ctx, r.Token,
		)
		if err != nil {
			return errors.Tag(err, "cannot resolve token")
		}
		if err = m.um.ConfirmEmail(ctx, d.UserId, d.Email); err != nil {
			if errors.IsNotFoundError(err) {
				return &errors.UnprocessableEntityError{
					Msg: "owner of email changed / email removed from account",
				}
			}
			return errors.Tag(err, "cannot confirm email")
		}
		return nil
	})
}

func (m *manager) ResendEmailConfirmation(ctx context.Context, r *types.ResendEmailConfirmationRequest) error {
	if err := r.Session.CheckIsLoggedIn(); err != nil {
		return err
	}
	r.Preprocess()
	if err := r.Validate(); err != nil {
		return err
	}
	userId := r.Session.User.Id

	u := &user.EmailsField{}
	if err := m.um.GetUser(ctx, userId, u); err != nil {
		return errors.Tag(err, "cannot get users emails")
	}

	found := false
	for _, e := range u.Emails {
		if e.Email == r.Email {
			found = true
			break
		}
	}
	if !found {
		return &errors.UnprocessableEntityError{
			Msg: "account does not hold given email",
		}
	}

	token, err := m.oTTm.NewForEmailConfirmation(ctx, userId, r.Email)
	if err != nil {
		return errors.Tag(err, "cannot get one time token")
	}

	e := &email.Email{
		Content: &email.CTAContent{
			PublicOptions: m.emailOptions.Public,
			Message: email.Message{
				fmt.Sprintf(
					"Please confirm that you have added a new email, %s, to your %s account.",
					r.Email, m.options.AppName,
				),
			},
			SecondaryMessage: email.Message{
				"If you did not request this, you can simply ignore this message.",
				fmt.Sprintf(
					"If you have any questions or trouble confirming your email address, please get in touch with our support team at %s.",
					m.options.AdminEmail,
				),
			},
			Title:   "Confirm Email",
			CTAText: "Confirm Email",
			CTAURL: m.options.SiteURL.
				WithPath("/user/emails/confirm").
				WithQuery(url.Values{
					"token": {string(token)},
				}),
		},
		Subject: "Confirm Email - " + m.options.AppName,
		To: &email.Identity{
			Address:     r.Email,
			DisplayName: r.Session.User.ToPublicUserInfo().DisplayName(),
		},
	}
	if err = e.Send(ctx, m.emailOptions.Send); err != nil {
		return errors.Tag(err, "cannot send email confirmation token")
	}
	return nil
}

func (m *manager) ConfirmEmailPage(_ context.Context, request *types.ConfirmEmailPageRequest, response *types.ConfirmEmailPageResponse) error {
	if err := request.Validate(); err != nil {
		return err
	}

	response.Data = &templates.UserConfirmEmailData{
		MarketingLayoutData: templates.MarketingLayoutData{
			JsLayoutData: templates.JsLayoutData{
				CommonData: templates.CommonData{
					Settings:    m.ps,
					SessionUser: request.Session.User,
					TitleLocale: "confirm_email",
				},
			},
		},
		Token: request.Token,
	}
	return nil
}
