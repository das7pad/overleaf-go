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

	"github.com/das7pad/overleaf-go/pkg/templates"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) Logout(ctx context.Context, request *types.LogoutRequest) error {
	if request.Session.IsLoggedIn() {
		if err := m.bumpEpoch(ctx, request.Session.User.Id); err != nil {
			return err
		}
	}
	return request.Session.Destroy(ctx)
}

func (m *manager) LogoutPage(_ context.Context, request *types.LogoutPageRequest, response *types.LogoutPageResponse) error {
	response.Data = &templates.UserLogoutData{
		MarketingLayoutData: templates.MarketingLayoutData{
			CommonData: templates.CommonData{
				Settings:    m.ps,
				SessionUser: request.Session.User,
				TitleLocale: "log_out",
				Viewport:    true,
			},
		},
	}
	return nil
}
