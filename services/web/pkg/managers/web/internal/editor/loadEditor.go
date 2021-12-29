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

package editor

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/jwt/loggedInUserJWT"
	"github.com/das7pad/overleaf-go/pkg/jwt/projectJWT"
	"github.com/das7pad/overleaf-go/pkg/jwt/userIdJWT"
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
		EditorConfigField: user.EditorConfigField{
			EditorConfig: user.EditorConfig{
				Mode:               "none",
				Theme:              "textmate",
				FontSize:           12,
				AutoComplete:       true,
				AutoPairDelimiters: true,
				PDFViewer:          "pdfjs",
				SyntaxValidation:   true,
			},
		},
	}
)

func (m *manager) genJWTLoggedInUser(userId primitive.ObjectID) (string, error) {
	c := m.jwtLoggedInUser.New().(*loggedInUserJWT.Claims)
	c.UserId = userId
	return m.jwtLoggedInUser.SetExpiryAndSign(c)
}

func (m *manager) genJWTSpelling(userId primitive.ObjectID) (string, error) {
	c := m.jwtSpelling.New().(*userIdJWT.Claims)
	c.UserId = userId
	return m.jwtSpelling.SetExpiryAndSign(c)
}

func (m *manager) genWSBootstrap(projectId primitive.ObjectID, u *user.WithPublicInfo) (types.WSBootstrap, error) {
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
	isAnonymous := userId.IsZero()
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
	u := &user.WithLoadEditorInfo{}
	var ownerFeatures *user.Features
	var authorizationDetails *project.AuthorizationDetails

	// Fan out 1 -- fetch primary mongo details
	eg, pCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		var err error
		p, err = m.pm.GetLoadEditorDetails(pCtx, projectId, userId)
		if err != nil {
			return errors.Tag(err, "cannot get project details")
		}

		authorizationDetails, err = p.GetPrivilegeLevel(
			userId, anonymousAccessToken,
		)
		if err != nil {
			return err
		}
		return nil
	})

	if isAnonymous {
		u = defaultUser
	} else {
		eg.Go(func() error {
			if err := m.um.GetUser(pCtx, userId, u); err != nil {
				return errors.Tag(err, "cannot get user details")
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	// Fan out 2 -- compute only for owned, unarchived projects
	eg, pCtx = errgroup.WithContext(ctx)

	if p.OwnerRef == userId {
		ownerFeatures = &u.Features
	} else {
		eg.Go(func() error {
			o := &user.FeaturesField{}
			if err := m.um.GetUser(pCtx, p.OwnerRef, o); err != nil {
				return errors.Tag(err, "cannot get project owner features")
			}
			ownerFeatures = &o.Features
			return nil
		})
	}

	eg.Go(func() error {
		b, err := m.genWSBootstrap(projectId, &u.WithPublicInfo)
		if err != nil {
			return errors.Tag(err, "cannot get wsBootstrap")
		}
		response.WSBootstrap = templates.WSBootstrap(b)
		return nil
	})

	if !p.Active {
		eg.Go(func() error {
			if err := m.dm.UnArchiveProject(pCtx, projectId); err != nil {
				return errors.Tag(err, "cannot un-archive project")
			}
			if err := m.pm.MarkAsActive(pCtx, projectId); err != nil {
				return errors.Tag(err, "cannot mark project as active")
			}
			return nil
		})
	}

	go func() {
		bCtx, done := context.WithTimeout(context.Background(), time.Second*10)
		defer done()
		if err := m.pm.MarkAsOpened(bCtx, projectId); err != nil {
			log.Println(
				errors.Tag(
					err, "cannot mark project as opened: "+projectId.Hex(),
				).Error(),
			)
		}
	}()

	if !isAnonymous {
		eg.Go(func() error {
			s, err := m.genJWTSpelling(userId)
			if err != nil {
				return errors.Tag(err, "cannot get spelling jwt")
			}
			response.JWTSpelling = s
			return nil
		})
		eg.Go(func() error {
			s, err := m.genJWTLoggedInUser(userId)
			if err != nil {
				return errors.Tag(err, "cannot get LoggedInUserJWT")
			}
			response.JWTLoggedInUser = s
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	{
		c := m.jwtProject.New().(*projectJWT.Claims)
		c.CompileGroup = ownerFeatures.CompileGroup
		c.EpochUser = u.Epoch
		c.ProjectId = projectId
		c.Timeout = ownerFeatures.CompileTimeout
		c.UserId = userId
		c.AuthorizationDetails = *authorizationDetails

		s, err := m.jwtProject.SetExpiryAndSign(c)
		if err != nil {
			return errors.Tag(err, "cannot get compile jwt")
		}
		response.JWTProject = s
	}

	t, err := p.GetRootFolder()
	if err != nil {
		return err
	}
	err = t.WalkDocs(func(e project.TreeElement, path sharedTypes.PathName) error {
		if e.GetId() == p.RootDocId {
			response.RootDocPath = clsiTypes.RootResourcePath(path)
			return project.AbortWalk
		}
		return nil
	})
	if err != nil {
		return err
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
	return nil
}
