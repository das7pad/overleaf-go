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
	l, err := m.pm.ListProjectsWithName(ctx, userId)
	if err != nil {
		return errors.Tag(err, "cannot get projects")
	}

	projects := make([]types.GetUserProjectsEntry, len(l))
	for i, p := range l {
		projects[i] = types.GetUserProjectsEntry{
			Id:   p.Id,
			Name: p.Name,
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
	u := project.ForProjectList{
		Tags:     make([]tag.Full, 0),
		Projects: make(project.List, 0),
	}
	{
		err := m.pm.GetProjectListDetails(ctx, userId, &u)
		if err != nil {
			return errors.Tag(err, "cannot get user with projects")
		}
	}

	projects := make([]*templates.ProjectListProjectView, len(u.Projects))
	for i, p := range u.Projects {
		authorizationDetails, err := p.GetPrivilegeLevelAuthenticated()
		if err != nil {
			return errors.New("listed project w/o access: " + p.Id.String())
		}
		projects[i] = &templates.ProjectListProjectView{
			Id:                p.Id,
			Name:              p.Name,
			LastUpdatedAt:     p.LastUpdatedAt,
			PublicAccessLevel: p.PublicAccessLevel,
			AccessLevel:       authorizationDetails.PrivilegeLevel,
			AccessSource:      authorizationDetails.AccessSource,
			Archived:          p.Archived,
			Trashed:           p.Trashed,
			OwnerRef:          p.OwnerId,
		}
		if authorizationDetails.IsRestrictedUser() {
			if p.LastUpdatedBy == userId {
				projects[i].LastUpdatedBy = u.User.WithPublicInfo
			} else {
				projects[i].LastUpdatedBy = user.WithPublicInfo{}
				projects[i].LastUpdatedBy.Id = p.LastUpdatedBy
			}
			projects[i].Owner = user.WithPublicInfo{}
			projects[i].Owner.Id = p.OwnerId
		} else {
			projects[i].LastUpdatedBy = p.LastUpdater
			projects[i].Owner = p.Owner
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
		Tags:            u.Tags,
		JWTLoggedInUser: jwtLoggedInUser,
		UserEmails:      u.User.ToUserEmails(),
	}
	return nil
}
