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

package editor

import (
	"context"
	"log"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/jwt/loggedInUserJWT"
	"github.com/das7pad/overleaf-go/pkg/jwt/projectJWT"
	"github.com/das7pad/overleaf-go/pkg/jwt/wsBootstrap"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/pkg/templates"
	clsiTypes "github.com/das7pad/overleaf-go/services/clsi/pkg/types"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

var (
	defaultUser = &user.WithLoadEditorInfo{
		EpochField: user.EpochField{
			Epoch: user.AnonymousUserEpoch,
		},
		EditorConfigField: user.EditorConfigField{
			EditorConfig: user.DefaultEditorConfig,
		},
	}
)

func (m *manager) genJWTLoggedInUser(userId sharedTypes.UUID) (string, error) {
	c := m.jwtLoggedInUser.New().(*loggedInUserJWT.Claims)
	c.UserId = userId
	return m.jwtLoggedInUser.SetExpiryAndSign(c)
}

func (m *manager) genWSBootstrap(projectId sharedTypes.UUID, u *user.WithPublicInfo) (types.WSBootstrap, error) {
	c := m.wsBootstrap.New().(*wsBootstrap.Claims)
	c.ProjectId = projectId
	c.User.Id = u.Id
	c.User.Email = u.Email
	c.User.FirstName = u.FirstName
	c.User.LastName = u.LastName

	blob, err := m.wsBootstrap.SetExpiryAndSign(c)
	if err != nil {
		return types.WSBootstrap{}, err
	}
	return types.WSBootstrap{
		JWT:       blob,
		ExpiresIn: int64(c.ExpiresIn().Seconds()),
	}, nil
}

func (m *manager) GetWSBootstrap(ctx context.Context, request *types.GetWSBootstrapRequest, response *types.GetWSBootstrapResponse) error {
	projectId := request.ProjectId
	userId := request.Session.User.Id
	token := request.Session.GetAnonTokenAccess(projectId)
	_, err := m.pm.GetAuthorizationDetails(ctx, projectId, userId, token)
	if err != nil {
		return err
	}
	u := request.Session.User.ToPublicUserInfo()
	b, err := m.genWSBootstrap(projectId, u)
	if err != nil {
		return errors.Tag(err, "cannot gen jwt")
	}
	*response = b
	return nil
}

func (m *manager) ProjectEditorPage(ctx context.Context, request *types.ProjectEditorPageRequest, res *types.ProjectEditorPageResponse) error {
	projectId := request.ProjectId
	userId := request.Session.User.Id
	isAnonymous := userId == (sharedTypes.UUID{})
	anonymousAccessToken := request.Session.GetAnonTokenAccess(projectId)
	if !isAnonymous {
		// Logged-in users must go through the join process first
		anonymousAccessToken = ""
	}

	response := &templates.EditorBootstrap{
		Anonymous:            isAnonymous,
		AnonymousAccessToken: anonymousAccessToken,
	}
	var p *project.LoadEditorViewPrivate
	var u *user.WithLoadEditorInfo
	var authorizationDetails *project.AuthorizationDetails
	{
		d, err := m.pm.GetLoadEditorDetails(
			ctx, projectId, userId, anonymousAccessToken,
		)
		if err != nil {
			return errors.Tag(err, "cannot get project/user details")
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

	if p.RootDoc.Id != (sharedTypes.UUID{}) {
		response.RootDocPath = clsiTypes.RootResourcePath(
			p.RootDoc.ResolvedPath,
		)
		p.LoadEditorViewPublic.RootDocId = p.RootDoc.Id
	}

	{
		b, err := m.genWSBootstrap(projectId, &u.WithPublicInfo)
		if err != nil {
			return errors.Tag(err, "cannot get wsBootstrap")
		}
		response.WSBootstrap = templates.WSBootstrap(b)
	}
	if !isAnonymous {
		s, err := m.genJWTLoggedInUser(userId)
		if err != nil {
			return errors.Tag(err, "cannot get LoggedInUserJWT")
		}
		response.JWTLoggedInUser = s
	}
	signedOptions := types.SignedCompileProjectRequestOptions{
		CompileGroup: p.OwnerFeatures.CompileGroup,
		ProjectId:    projectId,
		UserId:       userId,
		Timeout:      p.OwnerFeatures.CompileTimeout,
	}
	{
		c := m.jwtProject.New().(*projectJWT.Claims)
		c.EpochUser = u.Epoch
		c.AuthorizationDetails = *authorizationDetails
		c.SignedCompileProjectRequestOptions = signedOptions

		s, err := m.jwtProject.SetExpiryAndSign(c)
		if err != nil {
			return errors.Tag(err, "cannot get compile jwt")
		}
		response.JWTProject = s
	}
	if userId != m.options.SmokeTest.UserId {
		go func() {
			bCtx, done := context.WithTimeout(
				context.Background(), 10*time.Second,
			)
			defer done()
			err := m.cm.StartInBackground(bCtx, signedOptions, p.ImageName)
			if err != nil {
				log.Printf(
					"%s/%s: cannot start compile container: %s",
					projectId, userId, err.Error(),
				)
			}
		}()
	}

	response.IsRestrictedUser = authorizationDetails.IsRestrictedUser()
	response.Project = p.LoadEditorViewPublic
	response.User = *u
	if u.IsAdmin {
		response.AllowedImageNames = m.options.AllowedImageNames
	} else {
		response.AllowedImageNames = m.publicImageNames
	}
	res.Data = &templates.ProjectEditorData{
		AngularLayoutData: templates.AngularLayoutData{
			CommonData: templates.CommonData{
				Settings:              m.ps,
				RobotsNoindexNofollow: true,
				SessionUser:           request.Session.User,
				ThemeModifier:         u.EditorConfig.OverallTheme,
				Title:                 string(p.Name),
			},
		},
		EditorBootstrap: response,
	}
	go func() {
		bCtx, done := context.WithTimeout(context.Background(), 3*time.Second)
		defer done()
		err := m.pm.BumpLastOpened(bCtx, projectId)
		if err != nil {
			log.Printf(
				"%s/%s: cannot bump last opened time: %s",
				projectId, userId, err,
			)
		}
	}()
	return nil
}
