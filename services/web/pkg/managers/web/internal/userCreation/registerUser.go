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

package userCreation

import (
	"context"
	"log"
	"net/url"

	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/oneTimeToken"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/templates"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) RegisterUser(ctx context.Context, r *types.RegisterUserRequest, response *types.RegisterUserResponse) error {
	if m.options.RegistrationDisabled {
		return &errors.UnprocessableEntityError{
			Msg: "registration is disabled",
		}
	}
	r.Preprocess()
	if err := r.Validate(); err != nil {
		return err
	}

	var u *user.ForCreation
	var t oneTimeToken.OneTimeToken
	err := m.c.Tx(ctx, func(ctx context.Context, _ *edgedb.Tx) error {
		var err error
		u, err = m.createUser(ctx, r.Email, r.Password, r.IPAddress)
		if err != nil {
			return err
		}
		// TODO: merge into createUser
		t, err = m.oTTm.NewForEmailConfirmation(ctx, u.Id, r.Email)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if errors.GetCause(err) == user.ErrEmailAlreadyRegistered {
			response.SetCustomFormMessage("already-exists", err)
			return user.ErrEmailAlreadyRegistered
		}
		return err
	}

	go func() {
		errEmail := m.sendWelcomeEmail(ctx, r.Email, m.options.SiteURL.
			WithPath("/user/emails/confirm").
			WithQuery(url.Values{
				"token": {string(t)},
			}),
		)
		if errEmail != nil {
			log.Printf(
				"%s register user partial failure: %s",
				u.Id, errEmail.Error(),
			)
		}
	}()

	redirect, err := r.Session.Login(ctx, &u.ForSession, r.IPAddress)
	if err != nil {
		return err
	}
	response.RedirectTo = redirect
	return nil
}

func (m *manager) RegisterUserPage(_ context.Context, request *types.RegisterUserPageRequest, response *types.RegisterUserPageResponse) error {
	if request.Session.IsLoggedIn() {
		response.Redirect = "/project"
		return nil
	}
	request.SharedProjectData.Preprocess()

	response.Data = &templates.UserRegisterData{
		MarketingLayoutData: templates.MarketingLayoutData{
			CommonData: templates.CommonData{
				Settings:    m.ps,
				SessionUser: request.Session.User,
				TitleLocale: "register",
			},
		},
		SharedProjectData: request.SharedProjectData,
	}
	return nil
}
