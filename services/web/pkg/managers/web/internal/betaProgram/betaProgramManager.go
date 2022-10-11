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

package betaProgram

import (
	"context"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/templates"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	OptInBetaProgram(ctx context.Context, request *types.OptInBetaProgramRequest) error
	OptOutBetaProgram(ctx context.Context, request *types.OptOutBetaProgramRequest) error
	BetaProgramParticipatePage(ctx context.Context, request *types.BetaProgramParticipatePageRequest, response *types.BetaProgramParticipatePageResponse) error
}

func New(ps *templates.PublicSettings, um user.Manager) Manager {
	return &manager{
		ps: ps,
		um: um,
	}
}

type manager struct {
	ps *templates.PublicSettings
	um user.Manager
}

func (m *manager) OptInBetaProgram(ctx context.Context, request *types.OptInBetaProgramRequest) error {
	if err := request.Session.CheckIsLoggedIn(); err != nil {
		return err
	}
	return m.um.SetBetaProgram(ctx, request.Session.User.Id, true)
}

func (m *manager) OptOutBetaProgram(ctx context.Context, request *types.OptOutBetaProgramRequest) error {
	if err := request.Session.CheckIsLoggedIn(); err != nil {
		return err
	}
	return m.um.SetBetaProgram(ctx, request.Session.User.Id, false)
}

func (m *manager) BetaProgramParticipatePage(ctx context.Context, request *types.BetaProgramParticipatePageRequest, response *types.BetaProgramParticipatePageResponse) error {
	if err := request.Session.CheckIsLoggedIn(); err != nil {
		return err
	}
	u := user.BetaProgramField{}
	if err := m.um.GetUser(ctx, request.Session.User.Id, &u); err != nil {
		return errors.Tag(err, "cannot get user")
	}
	response.Data = &templates.BetaProgramParticipate{
		MarketingLayoutData: templates.MarketingLayoutData{
			CommonData: templates.CommonData{
				Settings:    m.ps,
				SessionUser: request.Session.User,
				TitleLocale: "sharelatex_beta_program",
				Viewport:    true,
			},
		},
		AlreadyInBetaProgram: u.BetaProgram,
	}
	return nil
}
