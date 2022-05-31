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

package login

import (
	"context"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/pkg/templates"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) ActivateUserPage(ctx context.Context, request *types.ActivateUserPageRequest, response *types.ActivateUserPageResponse) error {
	if err := request.Validate(); err != nil {
		return err
	}
	var userId sharedTypes.UUID
	{
		id, err := sharedTypes.ParseUUID(request.UserIdHex)
		if err != nil || id == (sharedTypes.UUID{}) {
			return &errors.ValidationError{Msg: "invalid user_id"}
		}
		userId = id
	}

	u := &user.ForActivateUserPage{}
	if err := m.um.GetUser(ctx, userId, u); err != nil {
		if errors.IsNotFoundError(err) {
			return &errors.UnprocessableEntityError{
				Msg: "user not found",
			}
		}
		return err
	}
	if u.LoginCount > 0 {
		response.Redirect = "/login"
		return nil
	}

	response.Data = &templates.UserActivateData{
		MarketingLayoutData: templates.MarketingLayoutData{
			CommonData: templates.CommonData{
				Settings:    m.ps,
				SessionUser: request.Session.User,
				TitleLocale: "activate_account",
			},
		},
		Email: u.Email,
		Token: request.Token,
	}
	return nil
}
