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

package openInOverleaf

import (
	"context"
	"encoding/json"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/templates"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) OpenInOverleafDocumentationPage(_ context.Context, request *types.OpenInOverleafDocumentationPageRequest, response *types.OpenInOverleafDocumentationPageResponse) error {
	response.Data = &templates.OpenInOverleafDocumentationData{
		MarketingLayoutData: templates.MarketingLayoutData{
			CommonData: templates.CommonData{
				Settings:    m.ps,
				Title:       "Open In Overleaf",
				SessionUser: request.Session.User,
			},
		},
	}
	return nil
}

func (m *manager) OpenInOverleafGatewayPage(_ context.Context, request *types.OpenInOverleafGatewayPageRequest, response *types.OpenInOverleafGatewayPageResponse) error {
	var params json.RawMessage
	if len(request.Query) != 0 {
		blob, err := json.Marshal(request.Query)
		if err != nil {
			return errors.Tag(err, "serialize params")
		}
		params = blob
	} else if request.Body != nil {
		params = request.Body
	}
	response.Data = &templates.OpenInOverleafGatewayData{
		MarketingLayoutData: templates.MarketingLayoutData{
			CommonData: templates.CommonData{
				Settings:    m.ps,
				Title:       "Open In Overleaf",
				SessionUser: request.Session.User,
			},
		},
		Params: params,
	}
	return nil
}
