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

package userCreation

import (
	"context"
	"fmt"

	"github.com/das7pad/overleaf-go/pkg/email"
	"github.com/das7pad/overleaf-go/pkg/email/pkg/spamSafe"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

func (m *manager) sendWelcomeEmail(to sharedTypes.Email, confirmEmailURL *sharedTypes.URL) error {
	withEmailHint := "."
	if spamSafe.IsSafeEmail(to) {
		withEmailHint = fmt.Sprintf(" with the email address '%s'.", to)
	}
	e := email.Email{
		Content: &email.CTAContent{
			PublicOptions: m.emailOptions.Public,
			Message: email.Message{
				fmt.Sprintf(
					"Thanks for signing up to %s!", m.appName,
				),
			},
			HelpLinks: []email.HelpLink{
				{
					Before: "If you ever get lost, you can ",
					URL:    m.siteURL.WithPath("/login"),
					Label:  "log in again",
					After:  withEmailHint,
				},
				{
					Before: "If you're new to LaTeX, take a look at our ",
					Label:  "Help Guides",
					URL: m.siteURL.WithPath(
						"/learn",
					),
					After: ".",
				},
			},
			CTAIntro: fmt.Sprintf(
				"Please also take a moment to confirm your email address for %s:",
				m.appName,
			),
			CTAURL:  confirmEmailURL,
			CTAText: "Confirm Email",
			Title:   "Welcome to " + m.appName,
		},
		Subject: "Welcome to " + m.appName,
		To: email.Identity{
			Address: to,
		},
	}
	if err := e.Send(context.Background(), m.emailOptions.Send); err != nil {
		return errors.Tag(err, "send welcome email")
	}
	return nil
}
