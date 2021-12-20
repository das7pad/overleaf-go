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

package templates

import (
	"github.com/das7pad/overleaf-go/pkg/models/oneTimeToken"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/session"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type UserConfirmEmailData struct {
	MarketingLayoutData
	Token oneTimeToken.OneTimeToken
}

type UserLoginData struct {
	MarketingLayoutData
}

type UserLogoutData struct {
	MarketingLayoutData
}

type UserReconfirmData struct {
	MarketingLayoutData
	Email sharedTypes.Email
}

type SharedProjectData struct {
	ProjectName project.Name `form:"project_name"`
	UserName    string       `form:"user_first_name"`
}

type UserRegisterDisabledData struct {
	MarketingLayoutData
	SharedProjectData SharedProjectData
}

type UserRestrictedData struct {
	MarketingLayoutData
}

type UserSessionsData struct {
	MarketingLayoutData
	CurrentSession session.OtherSessionData
	OtherSessions  []*session.OtherSessionData
}

type UserSetPasswordData struct {
	MarketingLayoutData
	Email              sharedTypes.Email
	PasswordResetToken oneTimeToken.OneTimeToken
}

type UserSettingsData struct {
	AngularLayoutData
	User user.ForSettingsPage
}
