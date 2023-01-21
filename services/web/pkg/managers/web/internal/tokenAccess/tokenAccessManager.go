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

package tokenAccess

import (
	"context"
	"time"

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

func New(options *types.Options, ps *templates.PublicSettings, pm project.Manager) Manager {
	n := options.RateLimits.LinkSharingTokenLookupConcurrency
	lookupSlots := make(chan struct{}, n)
	for i := int64(0); i < n; i++ {
		lookupSlots <- struct{}{}
	}
	return &manager{
		pm:          pm,
		ps:          ps,
		lookupSlots: lookupSlots,
	}
}

type manager struct {
	pm          project.Manager
	ps          *templates.PublicSettings
	lookupSlots chan struct{}
}

func (m *manager) GrantTokenAccessReadAndWrite(ctx context.Context, request *types.GrantTokenAccessRequest, response *types.GrantTokenAccessResponse) error {
	if err := request.Session.CheckIsLoggedIn(); err != nil {
		return err
	}
	return m.grantTokenAccess(
		ctx, request, response,
		sharedTypes.PrivilegeLevelReadAndWrite,
	)
}

func (m *manager) GrantTokenAccessReadOnly(ctx context.Context, request *types.GrantTokenAccessRequest, response *types.GrantTokenAccessResponse) error {
	return m.grantTokenAccess(
		ctx, request, response,
		sharedTypes.PrivilegeLevelReadOnly,
	)
}

func (m *manager) grantTokenAccess(ctx context.Context, request *types.GrantTokenAccessRequest, response *types.GrantTokenAccessResponse, privilegeLevel sharedTypes.PrivilegeLevel) error {
	userId := request.Session.User.Id
	token := request.Token
	p, fromToken, err := m.lookupProjectByToken(
		ctx, userId, privilegeLevel, token, 3*time.Second,
	)
	if err != nil {
		if errors.IsNotAuthorizedError(err) {
			response.RedirectTo = "/restricted"
			return &errors.NotAuthorizedError{}
		}
		return err
	}
	projectId := p.Id
	if request.Session.IsLoggedIn() {
		existing, _ := p.GetPrivilegeLevelAuthenticated()
		if fromToken.PrivilegeLevel.IsHigherThan(existing.PrivilegeLevel) {
			err = m.pm.GrantTokenAccess(
				ctx, projectId, userId, token, fromToken.PrivilegeLevel,
			)
			if err != nil {
				return errors.Tag(err, "cannot grant access")
			}
		}
	} else {
		request.Session.AddAnonTokenAccess(projectId, token)
	}
	response.RedirectTo = "/project/" + projectId.String()
	return nil
}

func (m *manager) TokenAccessPage(ctx context.Context, request *types.TokenAccessPageRequest, response *types.TokenAccessPageResponse) error {
	userId := request.Session.User.Id
	var postULR *sharedTypes.URL
	var privilegeLevel sharedTypes.PrivilegeLevel
	switch {
	case request.Token.ValidateReadOnly() == nil:
		privilegeLevel = sharedTypes.PrivilegeLevelReadOnly
		postULR = m.ps.SiteURL.
			WithPath("/api/grant/ro/" + string(request.Token))
	case request.Token.ValidateReadAndWrite() == nil:
		if err := request.Session.CheckIsLoggedIn(); err != nil {
			return err
		}
		privilegeLevel = sharedTypes.PrivilegeLevelReadAndWrite
		postULR = m.ps.SiteURL.
			WithPath("/api/grant/rw/" + string(request.Token))
	default:
		return &errors.NotFoundError{}
	}

	p, _, err := m.lookupProjectByToken(
		ctx, userId, privilegeLevel, request.Token, 500*time.Millisecond,
	)
	if err != nil {
		p = &project.ForTokenAccessDetails{}
		p.Name = "this project"
	}

	response.Data = &templates.ProjectTokenAccessData{
		AngularLayoutData: templates.AngularLayoutData{
			CommonData: templates.CommonData{
				Settings:    m.ps,
				TitleLocale: "join_project",
				Session:     request.Session.PublicData,
				Viewport:    true,
			},
		},
		PostURL:     postULR,
		ProjectName: p.Name,
	}
	return nil
}

func (m *manager) lookupProjectByToken(ctx context.Context, userId sharedTypes.UUID, privilegeLevel sharedTypes.PrivilegeLevel, token project.AccessToken, maxWait time.Duration) (*project.ForTokenAccessDetails, *project.AuthorizationDetails, error) {
	waitCtx, done := context.WithTimeout(ctx, maxWait)
	defer done()
	select {
	case <-waitCtx.Done():
		if err := ctx.Err(); err != nil {
			// Parent context cancelled.
			return nil, nil, err
		}
		return nil, nil, &errors.RateLimitedError{
			RetryIn: maxWait * 2,
		}
	case <-m.lookupSlots:
	}
	defer func() { m.lookupSlots <- struct{}{} }()
	p, d, err := m.pm.GetTokenAccessDetails(ctx, userId, privilegeLevel, token)
	if err != nil {
		return nil, nil, errors.Tag(err, "cannot get project")
	}
	return p, d, nil
}
