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

package projectList

import (
	"context"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/jwt/jwtHandler"
	"github.com/das7pad/overleaf-go/pkg/jwt/loggedInUserJWT"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/tag"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/pubSub/channel"
	"github.com/das7pad/overleaf-go/pkg/templates"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	GetUserProjects(ctx context.Context, request *types.GetUserProjectsRequest, response *types.GetUserProjectsResponse) error
	ProjectListPage(ctx context.Context, request *types.ProjectListPageRequest, response *types.ProjectListPageResponse) error
	ArchiveProject(ctx context.Context, request *types.ArchiveProjectRequest) error
	UnArchiveProject(ctx context.Context, request *types.UnArchiveProjectRequest) error
	TrashProject(ctx context.Context, request *types.TrashProjectRequest) error
	UnTrashProject(ctx context.Context, request *types.UnTrashProjectRequest) error
	RenameProject(ctx context.Context, request *types.RenameProjectRequest) error
}

func New(ps *templates.PublicSettings, editorEvents channel.Writer, pm project.Manager, tm tag.Manager, um user.Manager, jwtLoggedInUser jwtHandler.JWTHandler) Manager {
	return &manager{
		editorEvents:    editorEvents,
		pm:              pm,
		ps:              ps,
		tm:              tm,
		um:              um,
		jwtLoggedInUser: jwtLoggedInUser,
	}
}

type manager struct {
	editorEvents    channel.Writer
	pm              project.Manager
	ps              *templates.PublicSettings
	tm              tag.Manager
	um              user.Manager
	jwtLoggedInUser jwtHandler.JWTHandler
}

func (m *manager) GetUserProjects(ctx context.Context, request *types.GetUserProjectsRequest, response *types.GetUserProjectsResponse) error {
	if err := request.Session.CheckIsLoggedIn(); err != nil {
		return err
	}

	userId := request.Session.User.Id
	u := project.ForProjectList{}
	{
		err := m.pm.ListProjects(ctx, userId, &u)
		if err != nil {
			return errors.Tag(err, "cannot get user with projects")
		}
	}

	projects := make([]types.GetUserProjectsEntry, len(u.Projects))
	for i, p := range u.Projects {
		d, err2 := p.GetPrivilegeLevelAuthenticated(userId)
		if err2 != nil {
			return errors.New("listed project w/o access: " + p.Id.String())
		}
		projects[i] = types.GetUserProjectsEntry{
			Id:             p.Id,
			Name:           p.Name,
			PrivilegeLevel: d.PrivilegeLevel,
		}
	}
	response.Projects = projects
	return nil
}

func (m *manager) ProjectListPage(ctx context.Context, request *types.ProjectListPageRequest, response *types.ProjectListPageResponse) error {
	if err := request.Session.CheckIsLoggedIn(); err != nil {
		return err
	}
	userId := request.Session.User.Id
	u := project.ForProjectList{}
	{
		err := m.pm.ListProjects(ctx, userId, &u)
		if err != nil {
			return errors.Tag(err, "cannot get user with projects")
		}
	}

	projects := make([]*templates.ProjectListProjectView, len(u.Projects))
	for i, p := range u.Projects {
		authorizationDetails, err :=
			p.GetPrivilegeLevelAuthenticated(userId)
		if err != nil {
			return errors.New("listed project w/o access: " + p.Id.String())
		}
		projects[i] = &templates.ProjectListProjectView{
			Id:                p.Id,
			Name:              p.Name,
			LastUpdatedAt:     p.LastUpdatedAt,
			LastUpdatedBy:     p.LastUpdatedBy,
			PublicAccessLevel: p.PublicAccessLevel,
			AccessLevel:       authorizationDetails.PrivilegeLevel,
			AccessSource:      authorizationDetails.AccessSource,
			Archived:          p.ArchivedBy.Contains(userId),
			Trashed:           p.TrashedBy.Contains(userId),
			OwnerRef:          p.Owner.Id,
			Owner:             p.Owner.WithPublicInfo,
		}
		if authorizationDetails.IsRestrictedUser() {
			if projects[i].LastUpdatedBy.Id != userId {
				projects[i].LastUpdatedBy = user.WithPublicInfo{
					IdField: projects[i].LastUpdatedBy.IdField,
				}
			}
			projects[i].Owner = user.WithPublicInfo{
				IdField: p.Owner.IdField,
			}
		}
	}

	var jwtLoggedInUser string
	{
		c := m.jwtLoggedInUser.New().(*loggedInUserJWT.Claims)
		c.UserId = userId
		b, err := m.jwtLoggedInUser.SetExpiryAndSign(c)
		if err != nil {
			return errors.Tag(err, "cannot get LoggedInUserJWT")
		}
		jwtLoggedInUser = b
	}

	response.Data = &templates.ProjectListData{
		AngularLayoutData: templates.AngularLayoutData{
			CommonData: templates.CommonData{
				Settings:    m.ps,
				SessionUser: request.Session.User,
				TitleLocale: "your_projects",
			},
		},
		Projects:        projects,
		Tags:            u.Tags.Finalize(),
		JWTLoggedInUser: jwtLoggedInUser,
		UserEmails:      u.User.ToUserEmails(),
	}
	return nil
}
