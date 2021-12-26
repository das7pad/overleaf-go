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

package learn

import (
	"context"
	"html/template"
	"net/http"

	"github.com/das7pad/overleaf-go/services/web/pkg/templates"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	LearnPage(ctx context.Context, request *types.LearnPageRequest, response *types.LearnPageResponse) error
}

func New(options *types.Options, ps *templates.PublicSettings) Manager {
	return &manager{
		ps: ps,
	}
}

type manager struct {
	client *http.Client
	ps     *templates.PublicSettings
	cache  map[string]*template.HTML
}

func (m *manager) LearnPage(ctx context.Context, request *types.LearnPageRequest, response *types.LearnPageResponse) error {
	empty := template.HTML("")
	response.Data = &templates.LearnPageData{
		MarketingLayoutData: templates.MarketingLayoutData{
			CommonData: templates.CommonData{
				Settings:    m.ps,
				SessionUser: request.Session.User,
				Title:       "",
				Viewport:    true,
			},
		},
		PageContent:     &empty,
		ContentsContent: &empty,
	}
	return nil
}
