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

package admin

import (
	"context"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/services/web/pkg/templates"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) AdminManageSitePage(ctx context.Context, request *types.AdminManageSitePageRequest, response *types.AdminManageSitePageResponse) error {
	if err := request.Session.CheckIsAdmin(); err != nil {
		return err
	}

	messages, err := m.smm.GetAll(ctx)
	if err != nil {
		return errors.Tag(err, "cannot get system messages")
	}

	response.Data = &templates.AdminManageSiteData{
		MarketingLayoutData: templates.MarketingLayoutData{
			JsLayoutData: templates.JsLayoutData{
				CommonData: templates.CommonData{
					Settings:    m.ps,
					SessionUser: request.Session.User,
					Title:       "Manage Site",
				},
			},
		},
		SystemMessages: messages,
	}
	return nil
}
