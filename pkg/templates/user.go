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
	"strings"

	"github.com/das7pad/overleaf-go/pkg/email/pkg/spamSafe"
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

func (d *UserConfirmEmailData) Render() (string, error) {
	return render("user/confirmEmail.gohtml", 12*1024, d)
}

type UserLoginData struct {
	MarketingLayoutData
}

func (d *UserLoginData) Render() (string, error) {
	return render("user/login.gohtml", 10*1024, d)
}

type UserLogoutData struct {
	MarketingLayoutData
}

func (d *UserLogoutData) Render() (string, error) {
	return render("user/logout.gohtml", 10*1024, d)
}

type UserReconfirmData struct {
	MarketingLayoutData
	Email sharedTypes.Email
}

func (d *UserReconfirmData) Render() (string, error) {
	return render("user/reconfirm.gohtml", 9*1024, d)
}

type SharedProjectData struct {
	ProjectName project.Name `form:"project_name"`
	UserName    string       `form:"user_first_name"`
}

func (d *SharedProjectData) IsSet() bool {
	return d.ProjectName != "" && d.UserName != ""
}

func (d *SharedProjectData) Preprocess() {
	d.ProjectName = project.Name(strings.TrimSpace(string(d.ProjectName)))
	if d.ProjectName != "" && !spamSafe.IsSafeProjectName(d.ProjectName) {
		d.ProjectName = "their project"
	}
	d.UserName = strings.TrimSpace(d.UserName)
	if d.UserName != "" && !spamSafe.IsSafeUserName(d.UserName) {
		d.UserName = "A collaborator"
	}
}

type UserRegisterData struct {
	MarketingLayoutData
	SharedProjectData SharedProjectData
}

func (d *UserRegisterData) Render() (string, error) {
	return render("user/register.gohtml", 10*1024, d)
}

type UserRestrictedData struct {
	MarketingLayoutData
}

func (d *UserRestrictedData) Render() (string, error) {
	return render("user/restricted.gohtml", 11*1024, d)
}

type UserSessionsData struct {
	MarketingLayoutData
	CurrentSession *session.OtherSessionData
	OtherSessions  []*session.OtherSessionData
}

func (d *UserSessionsData) Render() (string, error) {
	n := 1024 * (13 + len(d.OtherSessions)*2/10)
	return render("user/sessions.gohtml", n, d)
}

type UserSetPasswordData struct {
	MarketingLayoutData
	Email              sharedTypes.Email
	PasswordResetToken oneTimeToken.OneTimeToken
}

func (d *UserSetPasswordData) Render() (string, error) {
	return render("user/setPassword.gohtml", 13*1024, d)
}

type UserActivateData struct {
	MarketingLayoutData
	Email sharedTypes.Email
	Token oneTimeToken.OneTimeToken
}

func (d *UserActivateData) Render() (string, error) {
	return render("user/activate.gohtml", 14*1024, d)
}

type UserPasswordResetData struct {
	MarketingLayoutData
	Email sharedTypes.Email
}

func (d *UserPasswordResetData) Render() (string, error) {
	return render("user/passwordReset.gohtml", 12*1024, d)
}

type UserSettingsData struct {
	AngularLayoutData
	User *user.ForSettingsPage
}

func (d *UserSettingsData) Render() (string, error) {
	n := 1024 * (24 +
		(len(d.User.FirstName)+len(d.User.LastName)+3*len(d.User.Email))/1024)
	return render("user/settings.gohtml", n, d)
}