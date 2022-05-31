// Golang port of Overleaf
// Copyright (C) 2021-2022 Jakob Ackermann <das7pad@outlook.com>
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
	"database/sql"

	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/models/oneTimeToken"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/templates"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/login"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	AdminCreateUser(ctx context.Context, r *types.AdminCreateUserRequest, response *types.AdminCreateUserResponse) error
	AdminRegisterUsersPage(_ context.Context, request *types.AdminRegisterUsersPageRequest, response *types.AdminRegisterUsersPageResponse) error
	RegisterUser(ctx context.Context, r *types.RegisterUserRequest, response *types.RegisterUserResponse) error
	RegisterUserPage(_ context.Context, request *types.RegisterUserPageRequest, response *types.RegisterUserPageResponse) error
}

func New(options *types.Options, ps *templates.PublicSettings, db *sql.DB, um user.Manager, lm login.Manager) Manager {
	return &manager{
		db:           db,
		emailOptions: options.EmailOptions(),
		lm:           lm,
		options:      options,
		oTTm:         oneTimeToken.New(db),
		ps:           ps,
		um:           um,
	}
}

type manager struct {
	c            *edgedb.Client
	db           *sql.DB
	emailOptions *types.EmailOptions
	lm           login.Manager
	options      *types.Options
	oTTm         oneTimeToken.Manager
	ps           *templates.PublicSettings
	um           user.Manager
}
