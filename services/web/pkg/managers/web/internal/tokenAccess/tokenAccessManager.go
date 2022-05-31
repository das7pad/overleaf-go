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

package tokenAccess

import (
	"context"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/pkg/templates"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	GrantTokenAccessReadAndWrite(ctx context.Context, request *types.GrantTokenAccessRequest, response *types.GrantTokenAccessResponse) error
	GrantTokenAccessReadOnly(ctx context.Context, request *types.GrantTokenAccessRequest, response *types.GrantTokenAccessResponse) error
	TokenAccessPage(ctx context.Context, request *types.TokenAccessPageRequest, response *types.TokenAccessPageResponse) error
}

func New(ps *templates.PublicSettings, pm project.Manager) Manager {
	return &manager{
		pm: pm,
		ps: ps,
	}
}

type manager struct {
	pm project.Manager
	ps *templates.PublicSettings
}

func (m *manager) GrantTokenAccessReadAndWrite(ctx context.Context, request *types.GrantTokenAccessRequest, response *types.GrantTokenAccessResponse) error {
	return m.grantTokenAccess(
		ctx, request, response,
		m.pm.GetProjectAccessForReadAndWriteToken, m.pm.GrantReadAndWriteTokenAccess,
	)
}

func (m *manager) GrantTokenAccessReadOnly(ctx context.Context, request *types.GrantTokenAccessRequest, response *types.GrantTokenAccessResponse) error {
	return m.grantTokenAccess(
		ctx, request, response,
		m.pm.GetProjectAccessForReadOnlyToken, m.pm.GrantReadOnlyTokenAccess,
	)
}

type getTokenAccess func(ctx context.Context, userId sharedTypes.UUID, token project.AccessToken) (*project.TokenAccessResult, error)
type grantAccess func(ctx context.Context, projectId sharedTypes.UUID, epoch int64, userId sharedTypes.UUID) error

func (m *manager) grantTokenAccess(ctx context.Context, request *types.GrantTokenAccessRequest, response *types.GrantTokenAccessResponse, getter getTokenAccess, granter grantAccess) error {
	userId := request.Session.User.Id
	token := request.Token
	for i := 0; i < 10; i++ {
		r, err := getter(ctx, userId, token)
		if err != nil {
			if errors.IsNotAuthorizedError(err) {
				response.RedirectTo = "/restricted"
				return nil
			}
			return errors.Tag(err, "cannot get project")
		}
		projectId := r.ProjectId
		if request.Session.IsLoggedIn() {
			if r.ShouldGrantHigherAccess() {
				err = granter(ctx, projectId, r.Epoch, userId)
				if err != nil {
					if errors.GetCause(err) == project.ErrEpochIsNotStable {
						continue
					}
					return errors.Tag(err, "cannot grant access")
				}
			}
		} else {
			request.Session.AddAnonTokenAccess(projectId, token)
		}
		response.RedirectTo = "/project/" + projectId.String()
		return nil
	}
	return project.ErrEpochIsNotStable
}

func (m *manager) TokenAccessPage(_ context.Context, request *types.TokenAccessPageRequest, response *types.TokenAccessPageResponse) error {
	var postULR *sharedTypes.URL
	if request.Token.ValidateReadOnly() == nil {
		postULR = m.ps.SiteURL.
			WithPath("/api/grant/ro/" + string(request.Token))
	} else if request.Token.ValidateReadAndWrite() == nil {
		postULR = m.ps.SiteURL.
			WithPath("/api/grant/rw/" + string(request.Token))
	} else {
		return &errors.NotFoundError{}
	}

	response.Data = &templates.ProjectTokenAccessData{
		AngularLayoutData: templates.AngularLayoutData{
			CommonData: templates.CommonData{
				Settings:    m.ps,
				TitleLocale: "join_project",
				SessionUser: request.Session.User,
				Viewport:    true,
			},
		},
		PostURL: postULR,
	}
	return nil
}
