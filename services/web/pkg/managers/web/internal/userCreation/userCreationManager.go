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

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/das7pad/overleaf-go/pkg/models/oneTimeToken"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/pkg/templates"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/login"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	CMDCreateUser(ctx context.Context, r *types.CMDCreateUserRequest, response *types.CMDCreateUserResponse) error
	RegisterUser(ctx context.Context, r *types.RegisterUserRequest, response *types.RegisterUserResponse) error
	RegisterUserPage(ctx context.Context, request *types.RegisterUserPageRequest, response *types.RegisterUserPageResponse) error
}

func New(options *types.Options, ps *templates.PublicSettings, db *pgxpool.Pool, um user.Manager, lm login.Manager) Manager {
	return &manager{
		emailOptions: options.EmailOptions(),
		lm:           lm,
		oTTm:         oneTimeToken.New(db),
		ps:           ps,
		um:           um,

		adminEmail:           options.AdminEmail,
		appName:              options.AppName,
		bcryptCost:           options.BcryptCost,
		registrationDisabled: options.RegistrationDisabled,
		siteURL:              options.SiteURL,
	}
}

type manager struct {
	emailOptions *types.EmailOptions
	lm           login.Manager
	oTTm         oneTimeToken.Manager
	ps           *templates.PublicSettings
	um           user.Manager

	adminEmail           sharedTypes.Email
	appName              string
	bcryptCost           int
	registrationDisabled bool
	siteURL              sharedTypes.URL
}
