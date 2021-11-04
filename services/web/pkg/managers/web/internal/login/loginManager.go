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
	"net/url"
	"time"

	"github.com/go-redis/redis/v8"
	"golang.org/x/crypto/bcrypt"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/jwt/jwtHandler"
	"github.com/das7pad/overleaf-go/pkg/jwt/loggedInUserJWT"
	"github.com/das7pad/overleaf-go/pkg/jwt/projectJWT"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/session"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	GetLoggedInUserJWT(ctx context.Context, request *types.GetLoggedInUserJWTRequest, response *types.GetLoggedInUserJWTResponse) error
	Login(ctx context.Context, request *types.LoginRequest, response *types.LoginResponse) error
	LogOut(ctx context.Context, request *types.LogoutRequest) error
}

func New(client redis.UniversalClient, um user.Manager, jwtLoggedInUser jwtHandler.JWTHandler) Manager {
	return &manager{
		client:          client,
		jwtLoggedInUser: jwtLoggedInUser,
		um:              um,
	}
}

type manager struct {
	client          redis.UniversalClient
	jwtLoggedInUser jwtHandler.JWTHandler
	um              user.Manager
}

var errNotLoggedIn = &errors.UnauthorizedError{}

func (m *manager) GetLoggedInUserJWT(_ context.Context, request *types.GetLoggedInUserJWTRequest, response *types.GetLoggedInUserJWTResponse) error {
	userId := request.Session.User.Id
	if userId.IsZero() {
		return errNotLoggedIn
	}
	c := m.jwtLoggedInUser.New().(*loggedInUserJWT.Claims)
	c.UserId = userId
	b, err := m.jwtLoggedInUser.SetExpiryAndSign(c)
	if err != nil {
		return errors.Tag(err, "cannot get LoggedInUserJWT")
	}
	*response = types.GetLoggedInUserJWTResponse(b)
	return nil
}

func (m *manager) LogOut(ctx context.Context, request *types.LogoutRequest) error {
	userId := request.Session.User.Id
	if !userId.IsZero() {
		_ = projectJWT.ClearUserField(ctx, m.client, userId)
		errBump := m.um.BumpEpoch(ctx, userId)
		errClearAgain := projectJWT.ClearUserField(ctx, m.client, userId)
		if errBump != nil {
			return errBump
		}
		if errClearAgain != nil {
			return errClearAgain
		}
	}
	return request.Session.Destroy(ctx)
}

func (m *manager) Login(ctx context.Context, r *types.LoginRequest, res *types.LoginResponse) error {
	r.Preprocess()
	if err := r.Validate(); err != nil {
		return err
	}
	u := &user.WithLoginInfo{}
	if err := m.um.GetUserByEmail(ctx, r.Email, u); err != nil {
		return errors.Tag(err, "cannot get user from mongo")
	}
	err := bcrypt.CompareHashAndPassword(
		[]byte(u.HashedPassword),
		[]byte(r.Password),
	)
	if err != nil {
		if err == bcrypt.ErrMismatchedHashAndPassword {
			return &errors.NotAuthorizedError{}
		}
		return errors.Tag(err, "cannot check user credentials")
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

	ip := r.IPAddress
	if err = m.um.TrackLogin(ctx, u.Id, ip); err != nil {
		return err
	}

	redirect := r.Session.PostLoginRedirect
	r.Session.SetNoAutoSave()
	r.Session.PostLoginRedirect = ""
	r.Session.User = &session.User{
		Id:             u.Id,
		IsAdmin:        u.IsAdmin,
		FirstName:      u.FirstName,
		LastName:       u.LastName,
		Email:          u.Email,
		Epoch:          u.Epoch,
		ReferralId:     u.ReferralId,
		IPAddress:      ip,
		SessionCreated: time.Now().UTC(),
	}
	if err = r.Session.Cycle(ctx); err != nil {
		return err
	}

	if redirect != "" {
		res.RedirectTo = redirect
	} else {
		res.RedirectTo = "/project"
	}
	return nil
}
