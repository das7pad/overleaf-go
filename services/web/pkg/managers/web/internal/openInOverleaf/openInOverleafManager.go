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

package openInOverleaf

import (
	"context"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/pkg/templates"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/linkedURLProxy"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/projectUpload"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	OpenInOverleaf(ctx context.Context, request *types.OpenInOverleafRequest, response *types.CreateProjectResponse) error
	OpenInOverleafGatewayPage(ctx context.Context, request *types.OpenInOverleafGatewayPageRequest, response *types.OpenInOverleafGatewayPageResponse) error
	OpenInOverleafDocumentationPage(ctx context.Context, request *types.OpenInOverleafDocumentationPageRequest, response *types.OpenInOverleafDocumentationPageResponse) error
}

func New(options *types.Options, ps *templates.PublicSettings, proxy linkedURLProxy.Manager, pum projectUpload.Manager) Manager {
	raw := options.SiteURL.WithPath("/learn")
	learnURL := sharedTypes.Snapshot(raw.String())
	return &manager{
		learnURL: learnURL,
		proxy:    proxy,
		ps:       ps,
		pum:      pum,
	}
}

type manager struct {
	learnURL sharedTypes.Snapshot
	proxy    linkedURLProxy.Manager
	ps       *templates.PublicSettings
	pum      projectUpload.Manager
}

func (m *manager) OpenInOverleaf(ctx context.Context, request *types.OpenInOverleafRequest, response *types.CreateProjectResponse) error {
	if err := request.Session.CheckIsLoggedIn(); err != nil {
		return err
	}
	request.Preprocess()
	if err := request.Validate(); err != nil {
		return err
	}

	if request.ZipURL != nil {
		return m.createFromZip(ctx, request, response)
	} else {
		return m.createFromSnippets(ctx, request, response)
	}
}
