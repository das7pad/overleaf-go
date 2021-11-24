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

	"github.com/das7pad/overleaf-go/pkg/jwt/jwtHandler"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	ClearSessions(ctx context.Context, request *types.ClearSessionsRequest) error
	GetLoggedInUserJWT(ctx context.Context, request *types.GetLoggedInUserJWTRequest, response *types.GetLoggedInUserJWTResponse) error
	Login(ctx context.Context, request *types.LoginRequest, response *types.LoginResponse) error
	Logout(ctx context.Context, request *types.LogoutRequest) error
}

func New(options *types.Options, client redis.UniversalClient, um user.Manager, jwtLoggedInUser jwtHandler.JWTHandler) Manager {
	return &manager{
		client:          client,
		emailOptions:    options.EmailOptions(),
		jwtLoggedInUser: jwtLoggedInUser,
		options:         options,
		um:              um,
	}
}

type manager struct {
	client          redis.UniversalClient
	emailOptions    *types.EmailOptions
	jwtLoggedInUser jwtHandler.JWTHandler
	options         *types.Options
	um              user.Manager
}
