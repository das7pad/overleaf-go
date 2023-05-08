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
	"net/url"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/pkg/templates"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) Login(ctx context.Context, r *types.LoginRequest, res *types.LoginResponse) error {
	r.Preprocess()
	if err := r.Validate(); err != nil {
		return err
	}
	u := user.WithLoginInfo{}
	if err := m.um.GetUserByEmail(ctx, r.Email, &u); err != nil {
		if errors.IsNotFoundError(err) {
			res.SetCustomFormMessage("user-not-found", err)
		}
		return errors.Tag(err, "get user from db")
	}
	if err := CheckPassword(u.HashedPasswordField, r.Password); err != nil {
		if errors.IsNotAuthorizedError(err) {
			res.SetCustomFormMessage("invalid-password", err)
		}
		return err
	}

	if u.MustReconfirm {
		to := url.URL{}
		to.Path = "/user/reconfirm"
		q := url.Values{}
		q.Set("email", string(u.Email))
		to.RawQuery = q.Encode()
		res.RedirectTo = to.String()
		return nil
	}

	eg, pCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		err := m.um.TrackLogin(pCtx, u.Id, u.Epoch, r.IPAddress)
		if err != nil {
			return errors.Tag(err, "track login")
		}
		return nil
	})
	var triggerSessionCleanup func()
	eg.Go(func() error {
		var err error
		res.RedirectTo, triggerSessionCleanup, err = r.Session.PrepareLogin(
			pCtx, u.ForSession, r.IPAddress,
		)
		return err
	})
	if err := eg.Wait(); err != nil {
		return err
	}
	triggerSessionCleanup()
	return nil
}

func (m *manager) LoginPage(_ context.Context, request *types.LoginPageRequest, response *types.LoginPageResponse) error {
	if request.Session.IsLoggedIn() {
		response.Redirect = "/project"
		return nil
	}
	if request.Referrer != "" {
		u, err := sharedTypes.ParseAndValidateURL(request.Referrer)
		if err == nil && strings.TrimSuffix(u.Path, "/") == "/docs" {
			if request.Session.PostLoginRedirect == "" {
				request.Session.PostLoginRedirect = "/docs"
			}
		}
	}
	response.Data = &templates.UserLoginData{
		MarketingLayoutData: templates.MarketingLayoutData{
			CommonData: templates.CommonData{
				Settings:    m.ps,
				Session:     request.Session.PublicData,
				TitleLocale: "login",
				Viewport:    true,
			},
		},
	}
	return nil
}
