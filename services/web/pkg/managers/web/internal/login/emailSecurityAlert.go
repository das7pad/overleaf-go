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
	"strings"
	"time"

	"github.com/das7pad/overleaf-go/pkg/email"
	"github.com/das7pad/overleaf-go/pkg/email/pkg/spamSafe"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/user"
)

func (m *manager) emailSecurityAlert(ctx context.Context, u *user.WithPublicInfo, action, actionDescribed string) error {
	now := time.Now().UTC().Format("Monday 02 January 2006 at 15:04 MST")
	msg1 := fmt.Sprintf(
		"We are writing to let you know that %s on %s.",
		actionDescribed, now,
	)
	msg2 := "If this was you, you can ignore this email."
	msg3 := fmt.Sprintf(
		"If this was not you, we recommend getting in touch with our support team at %s to report this as potentially suspicious activity on your account.",
		m.options.AdminEmail,
	)
	e := email.Email{
		Content: &email.NoCTAContent{
			PublicOptions: m.emailOptions.Public,
			Message:       email.Message{msg1, msg2, msg3},
			Title:         strings.ToTitle(string(action[0])) + action[1:],
			HelpLinks: []email.HelpLink{
				{
					Before: "We also encourage you to read our ",
					URL: m.options.SiteURL.WithPath(
						"/learn/how-to/Keeping_your_account_secure",
					),
					Label: "quick guide",
					After: fmt.Sprintf(
						" to keeping your %s account safe.",
						m.options.AppName,
					),
				},
			},
		},
		Subject: m.options.AppName + " security note: " + action,
		To: &email.Identity{
			Address: u.Email,
			DisplayName: spamSafe.GetSafeUserName(
				u.DisplayName(), "",
			),
		},
	}
	if err := e.Send(ctx, m.emailOptions.Send); err != nil {
		return errors.Tag(err, "cannot send security alert email")
	}
	return nil
}
