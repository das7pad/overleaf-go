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
	"github.com/das7pad/overleaf-go/pkg/templates"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) ConfirmEmail(ctx context.Context, r *types.ConfirmEmailRequest) error {
	if err := r.Validate(); err != nil {
		return err
	}
	err := m.oTTm.ResolveAndExpireEmailConfirmationToken(ctx, r.Token)
	if err != nil {
		return errors.Tag(err, "resolve token")
	}
	return nil
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

	token, err := m.oTTm.NewForEmailConfirmation(ctx, userId, r.Email)
	if err != nil {
		return errors.Tag(err, "get one time token")
	}

	u := r.Session.User.ToPublicUserInfo()
	e := email.Email{
		Content: &email.CTAContent{
			PublicOptions: m.emailOptions.Public,
			Message: email.Message{
				fmt.Sprintf(
					"Please confirm that you have added a new email, %s, to your %s account.",
					r.Email, m.appName,
				),
			},
			SecondaryMessage: email.Message{
				"If you did not request this, you can simply ignore this message.",
				fmt.Sprintf(
					"If you have any questions or trouble confirming your email address, please get in touch with our support team at %s.",
					m.adminEmail,
				),
			},
			Title:   "Confirm Email",
			CTAText: "Confirm Email",
			CTAURL: m.siteURL.
				WithPath("/user/emails/confirm").
				WithQuery(url.Values{
					"token": {string(token)},
				}),
		},
		Subject: "Confirm Email - " + m.appName,
		To: email.Identity{
			Address:     r.Email,
			DisplayName: u.DisplayName(),
		},
	}
	if err = e.Send(ctx, m.emailOptions.Send); err != nil {
		return errors.Tag(err, "send email confirmation token")
	}
	return nil
}

func (m *manager) ConfirmEmailPage(_ context.Context, request *types.ConfirmEmailPageRequest, response *types.ConfirmEmailPageResponse) error {
	if err := request.Validate(); err != nil {
		return err
	}

	response.Data = &templates.UserConfirmEmailData{
		MarketingLayoutData: templates.MarketingLayoutData{
			CommonData: templates.CommonData{
				Settings:    m.ps,
				Session:     request.Session.PublicData,
				TitleLocale: "confirm_email",
			},
		},
		Token: request.Token,
	}
	return nil
}
