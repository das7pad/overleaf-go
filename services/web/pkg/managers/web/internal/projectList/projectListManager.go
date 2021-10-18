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

	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/jwt/jwtHandler"
	"github.com/das7pad/overleaf-go/pkg/jwt/userIdJWT"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/tag"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	ProjectList(ctx context.Context, request *types.ProjectListRequest, response *types.ProjectListResponse) error
}

func New(options *types.Options, pm project.Manager, tm tag.Manager, um user.Manager) Manager {
	return &manager{
		pm:               pm,
		tm:               tm,
		um:               um,
		jwtNotifications: userIdJWT.New(options.JWT.Notifications),
	}
}

type manager struct {
	pm               project.Manager
	tm               tag.Manager
	um               user.Manager
	jwtNotifications jwtHandler.JWTHandler
}

func (m *manager) ProjectList(ctx context.Context, request *types.ProjectListRequest, response *types.ProjectListResponse) error {
	userId := request.UserId
	eg, pCtx := errgroup.WithContext(ctx)

	u := &user.ProjectListViewCaller{}
	eg.Go(func() error {
		if err := m.um.GetUser(pCtx, userId, u); err != nil {
			return errors.Tag(err, "cannot get user")
		}
		response.UserEmails = u.ToUserEmails()
		return nil
	})

	eg.Go(func() error {
		projectsRaw, err := m.pm.ListProjects(pCtx, userId)
		if err != nil {
			return errors.Tag(err, "cannot get projects")
		}
		projects := make([]*types.ProjectListProjectView, len(projectsRaw))
		response.Projects = projects

		lookUpUserIds := make(map[primitive.ObjectID]bool)
		for i, p := range projectsRaw {
			authorizationDetails, err2 :=
				p.GetPrivilegeLevelAuthenticated(userId)
			if err2 != nil {
				return errors.New("listed project w/o access: " + p.Id.Hex())
			}
			isArchived := p.ArchivedBy.Contains(userId)
			isTrashed := p.TrashedBy.Contains(userId)
			projects[i] = &types.ProjectListProjectView{
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
			if !authorizationDetails.IsRestrictedUser {
				lookUpUserIds[p.OwnerRef] = true
				lookUpUserIds[p.LastUpdatedBy] = true
			}
		}
		// Delete marker for missing LastUpdatedBy field.
		delete(lookUpUserIds, primitive.NilObjectID)
		// Delete marker for caller, we process it later.
		delete(lookUpUserIds, userId)

		if len(lookUpUserIds) == 0 {
			// Fast path: no collaborators.
			return nil
		}

		flatUserIds := make([]primitive.ObjectID, 0, len(lookUpUserIds))
		for id := range lookUpUserIds {
			flatUserIds = append(flatUserIds, id)
		}
		flatUsers, err := m.um.GetUsersWithPublicInfo(pCtx, flatUserIds)
		if err != nil {
			return errors.Tag(err, "cannot get other user details")
		}
		users := make(
			map[primitive.ObjectID]*user.WithPublicInfo, len(flatUsers),
		)
		for i := range flatUsers {
			usr := flatUsers[i]
			users[usr.Id] = &usr
		}
		for _, p := range projects {
			p.LastUpdatedBy = users[p.LastUpdatedByUserId]
			p.Owner = users[p.OwnerRef]
		}
		return nil
	})

	eg.Go(func() error {
		tags, err := m.tm.GetAll(ctx, userId)
		if err != nil {
			return errors.Tag(err, "cannot get tags")
		}
		response.Tags = tags
		return nil
	})

	eg.Go(func() error {
		c := m.jwtNotifications.New().(*userIdJWT.Claims)
		c.UserId = userId
		b, err := m.jwtNotifications.SetExpiryAndSign(c)
		if err != nil {
			return errors.Tag(err, "cannot get notifications jwt")
		}
		response.JWTNotifications = b
		return nil
	})

	if err := eg.Wait(); err != nil {
		return err
	}

	for _, p := range response.Projects {
		if p.OwnerRef == userId {
			p.Owner = &u.WithPublicInfo
		}
		if p.LastUpdatedByUserId == userId {
			p.LastUpdatedBy = &u.WithPublicInfo
		}
	}
	return nil
}