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

	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/das7pad/overleaf-go/pkg/jwt/jwtHandler"
	"github.com/das7pad/overleaf-go/pkg/models/oneTimeToken"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/session"
	"github.com/das7pad/overleaf-go/services/web/pkg/templates"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	ChangeEmailAddress(ctx context.Context, r *types.ChangeEmailAddressRequest) error
	ChangePassword(ctx context.Context, r *types.ChangePasswordRequest, response *types.ChangePasswordResponse) error
	ClearSessions(ctx context.Context, request *types.ClearSessionsRequest) error
	ConfirmEmail(ctx context.Context, r *types.ConfirmEmailRequest) error
	GetLoggedInUserJWT(ctx context.Context, request *types.GetLoggedInUserJWTRequest, response *types.GetLoggedInUserJWTResponse) error
	Login(ctx context.Context, request *types.LoginRequest, response *types.LoginResponse) error
	LoginPage(_ context.Context, request *types.LoginPageRequest, response *types.LoginPageResponse) error
	Logout(ctx context.Context, request *types.LogoutRequest) error
	LogoutPage(_ context.Context, request *types.LogoutPageRequest, response *types.LogoutPageResponse) error
	RequestPasswordReset(ctx context.Context, r *types.RequestPasswordResetRequest) error
	ResendEmailConfirmation(ctx context.Context, r *types.ResendEmailConfirmationRequest) error
	SetPassword(ctx context.Context, r *types.SetPasswordRequest, response *types.SetPasswordResponse) error
	SetUserName(ctx context.Context, r *types.SetUserName) error
}

func New(options *types.Options, ps *templates.PublicSettings, client redis.UniversalClient, db *mongo.Database, um user.Manager, jwtLoggedInUser jwtHandler.JWTHandler, sm session.Manager) Manager {
	return &manager{
		client:          client,
		db:              db,
		emailOptions:    options.EmailOptions(),
		jwtLoggedInUser: jwtLoggedInUser,
		options:         options,
		oTTm:            oneTimeToken.New(db),
		ps:              ps,
		sm:              sm,
		um:              um,
	}
}

type manager struct {
	client          redis.UniversalClient
	db              *mongo.Database
	emailOptions    *types.EmailOptions
	jwtLoggedInUser jwtHandler.JWTHandler
	options         *types.Options
	oTTm            oneTimeToken.Manager
	ps              *templates.PublicSettings
	sm              session.Manager
	um              user.Manager
}
