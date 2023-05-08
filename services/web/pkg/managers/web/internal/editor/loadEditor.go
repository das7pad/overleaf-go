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

package editor

import (
	"context"
	"log"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/pkg/templates"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

var defaultUser = &user.WithLoadEditorInfo{
	EpochField: user.EpochField{
		Epoch: user.AnonymousUserEpoch,
	},
	EditorConfigField: user.EditorConfigField{
		EditorConfig: user.DefaultEditorConfig,
	},
}

func (m *manager) ProjectEditorPage(ctx context.Context, request *types.ProjectEditorPageRequest, res *types.ProjectEditorPageResponse) error {
	if err := request.Validate(); err != nil {
		return err
	}
	projectId := request.ProjectId
	userId := request.Session.User.Id
	isAnonymous := userId.IsZero()
	anonymousAccessToken := request.Session.GetAnonTokenAccess(projectId)

	response := templates.EditorBootstrap{
		Anonymous:            isAnonymous,
		AnonymousAccessToken: anonymousAccessToken,
		DetachRole:           request.DetachRole,
	}
	var p *project.LoadEditorViewPrivate
	var u *user.WithLoadEditorInfo
	var authorizationDetails *project.AuthorizationDetails
	{
		d, err := m.pm.GetLoadEditorDetails(
			ctx, projectId, userId, anonymousAccessToken,
		)
		if err != nil {
			return errors.Tag(err, "get project/user details")
		}
		p = &d.Project
		if isAnonymous {
			u = defaultUser
		} else {
			u = &d.User
		}

		authorizationDetails, err = p.GetPrivilegeLevel(
			userId, anonymousAccessToken,
		)
		if err != nil {
			return err
		}
	}

	if !isAnonymous {
		c := m.jwtLoggedInUser.New()
		c.UserId = userId
		s, err := m.jwtLoggedInUser.SetExpiryAndSign(c)
		if err != nil {
			return errors.Tag(err, "get LoggedInUserJWT")
		}
		response.JWTLoggedInUser = s
		response.SystemMessages, _ = m.smm.GetAllCachedOnly(userId)
	}
	signedOptions := sharedTypes.SignedCompileProjectRequestOptions{
		CompileGroup: p.OwnerFeatures.CompileGroup,
		ProjectId:    projectId,
		UserId:       userId,
		Timeout:      p.OwnerFeatures.CompileTimeout,
	}
	{
		c := m.jwtProject.New()
		c.EpochUser = u.Epoch
		c.AuthorizationDetails = *authorizationDetails
		c.SignedCompileProjectRequestOptions = signedOptions

		s, err := m.jwtProject.SetExpiryAndSign(c)
		if err != nil {
			return errors.Tag(err, "get compile jwt")
		}
		response.JWTProject = s
	}
	if userId != m.smokeTestUserId {
		go func() {
			bCtx, done := context.WithTimeout(
				context.Background(), 10*time.Second,
			)
			defer done()
			err := m.cm.StartInBackground(bCtx, signedOptions, p.ImageName)
			if err != nil {
				log.Printf(
					"%s/%s: start compile container: %s",
					projectId, userId, err.Error(),
				)
			}
		}()
	}

	response.IsRestrictedUser = authorizationDetails.IsRestrictedUser()
	response.Project = p.LoadEditorViewPublic
	response.RootDocPath = p.RootDoc.Path
	response.User = *u
	response.AllowedImageNames = m.frontendAllowedImageNames
	res.Data = &templates.ProjectEditorData{
		AngularLayoutData: templates.AngularLayoutData{
			CommonData: templates.CommonData{
				Settings:              m.ps,
				RobotsNoindexNofollow: true,
				Session:               request.Session.PublicData,
				ThemeModifier:         u.EditorConfig.OverallTheme,
				Title:                 string(p.Name),
			},
		},
		EditorBootstrap: &response,
	}
	go func() {
		bCtx, done := context.WithTimeout(context.Background(), 3*time.Second)
		defer done()
		err := m.pm.BumpLastOpened(bCtx, projectId)
		if err != nil {
			log.Printf(
				"%s/%s: bump last opened time: %s",
				projectId, userId, err,
			)
		}
	}()
	return nil
}

func (m *manager) ProjectEditorDetached(ctx context.Context, request *types.ProjectEditorDetachedPageRequest, res *types.ProjectEditorDetachedPageResponse) error {
	projectId := request.ProjectId
	userId := request.Session.User.Id
	isAnonymous := userId.IsZero()
	anonymousAccessToken := request.Session.GetAnonTokenAccess(projectId)

	d, err := m.pm.GetLoadEditorDetails(
		ctx, projectId, userId, anonymousAccessToken,
	)
	if err != nil {
		return errors.Tag(err, "get project/user details")
	}
	if isAnonymous {
		d.User = *defaultUser
	}

	authorizationDetails, err := d.Project.GetPrivilegeLevel(
		userId, anonymousAccessToken,
	)
	if err != nil {
		return err
	}

	res.Data = &templates.ProjectEditorDetachedData{
		MarketingLayoutData: templates.MarketingLayoutData{
			CommonData: templates.CommonData{
				Settings:              m.ps,
				RobotsNoindexNofollow: true,
				Session:               request.Session.PublicData,
				ThemeModifier:         d.User.EditorConfig.OverallTheme,
				Title:                 string(d.Project.Name),
			},
		},
		EditorBootstrap: &templates.EditorBootstrap{
			AllowedImageNames:    m.frontendAllowedImageNames,
			Anonymous:            isAnonymous,
			AnonymousAccessToken: anonymousAccessToken,
			IsRestrictedUser:     authorizationDetails.IsRestrictedUser(),
			Project:              d.Project.LoadEditorViewPublic,
			RootDocPath:          d.Project.RootDoc.Path,
			User:                 d.User,
			DetachRole:           "detached",
		},
	}
	return nil
}
