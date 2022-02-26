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

package projectList

import (
	"context"

	"github.com/edgedb/edgedb-go"
	"golang.org/x/sync/errgroup"

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
	projectsRaw, err := m.pm.ListProjects(ctx, userId)
	if err != nil {
		return errors.Tag(err, "cannot get projects")
	}
	projects := make([]types.GetUserProjectsEntry, len(projectsRaw))
	for i, p := range projectsRaw {
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
	eg, pCtx := errgroup.WithContext(ctx)

	u := &user.ProjectListViewCaller{}
	eg.Go(func() error {
		if err := m.um.GetUser(pCtx, userId, u); err != nil {
			return errors.Tag(err, "cannot get user")
		}
		return nil
	})

	var projects []*templates.ProjectListProjectView
	eg.Go(func() error {
		projectsRaw, err := m.pm.ListProjects(pCtx, userId)
		if err != nil {
			return errors.Tag(err, "cannot get projects")
		}
		projects = make([]*templates.ProjectListProjectView, len(projectsRaw))

		lookUpUserIds := make(user.UniqUserIds)
		for i, p := range projectsRaw {
			authorizationDetails, err2 :=
				p.GetPrivilegeLevelAuthenticated(userId)
			if err2 != nil {
				return errors.New("listed project w/o access: " + p.Id.String())
			}
			isArchived := p.ArchivedBy.Contains(userId)
			isTrashed := p.TrashedBy.Contains(userId)
			projects[i] = &templates.ProjectListProjectView{
				Id:                  p.Id,
				Name:                p.Name,
				LastUpdatedAt:       p.LastUpdatedAt,
				LastUpdatedByUserId: p.LastUpdatedBy,
				PublicAccessLevel:   p.PublicAccessLevel,
				AccessLevel:         authorizationDetails.PrivilegeLevel,
				AccessSource:        authorizationDetails.AccessSource,
				Archived:            isArchived,
				Trashed:             isTrashed && !isArchived,
				OwnerRef:            p.OwnerRef,
			}
			if !authorizationDetails.IsRestrictedUser() {
				lookUpUserIds[p.OwnerRef] = true
				lookUpUserIds[p.LastUpdatedBy] = true
			}
		}
		// Delete marker for missing LastUpdatedBy field.
		delete(lookUpUserIds, edgedb.UUID{})
		// Delete marker for caller, we process it later.
		delete(lookUpUserIds, userId)

		if len(lookUpUserIds) == 0 {
			// Fast path: no collaborators.
			return nil
		}

		users, err := m.um.GetUsersForBackFilling(pCtx, lookUpUserIds)
		if err != nil {
			return errors.Tag(err, "cannot get other user details")
		}
		for _, p := range projects {
			p.LastUpdatedBy = users[p.LastUpdatedByUserId]
			p.Owner = users[p.OwnerRef]
		}
		return nil
	})

	var tags []tag.Full
	eg.Go(func() error {
		var err error
		tags, err = m.tm.GetAll(pCtx, userId)
		if err != nil {
			return errors.Tag(err, "cannot get tags")
		}
		return nil
	})

	var jwtLoggedInUser string
	eg.Go(func() error {
		c := m.jwtLoggedInUser.New().(*loggedInUserJWT.Claims)
		c.UserId = userId
		b, err := m.jwtLoggedInUser.SetExpiryAndSign(c)
		if err != nil {
			return errors.Tag(err, "cannot get LoggedInUserJWT")
		}
		jwtLoggedInUser = b
		return nil
	})

	if err := eg.Wait(); err != nil {
		return err
	}

	for _, p := range projects {
		if p.OwnerRef == userId {
			p.Owner = &u.WithPublicInfo
		}
		if p.LastUpdatedByUserId == userId {
			p.LastUpdatedBy = &u.WithPublicInfo
		}
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
		Tags:            tags,
		JWTLoggedInUser: jwtLoggedInUser,
		UserEmails:      u.ToUserEmails(),
	}
	return nil
}
