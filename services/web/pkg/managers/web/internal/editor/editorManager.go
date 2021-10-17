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
	"github.com/das7pad/overleaf-go/pkg/jwt/compileJWT"
	"github.com/das7pad/overleaf-go/pkg/jwt/jwtHandler"
	"github.com/das7pad/overleaf-go/pkg/jwt/userIdJWT"
	"github.com/das7pad/overleaf-go/pkg/jwt/wsBootstrap"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/services/docstore/pkg/managers/docstore"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	LoadEditor(ctx context.Context, request *types.LoadEditorRequest, response *types.LoadEditorResponse) error
}

func New(options *types.Options, pm project.Manager, um user.Manager, dm docstore.Manager, compileJWTHandler jwtHandler.JWTHandler) Manager {
	return &manager{
		dm:          dm,
		jwtCompile:  compileJWTHandler,
		jwtSpelling: userIdJWT.New(options.JWT.Spelling),
		pm:          pm,
		um:          um,
		wsBootstrap: wsBootstrap.New(options.JWT.RealTime),
	}
}

type manager struct {
	dm          docstore.Manager
	jwtCompile  jwtHandler.JWTHandler
	jwtSpelling jwtHandler.JWTHandler
	pm          project.Manager
	um          user.Manager
	wsBootstrap jwtHandler.JWTHandler
}

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

func (m *manager) genJWTCompile(ctx context.Context, projectId, userId primitive.ObjectID, ownerFeatures user.Features) (string, error) {
	c := m.jwtCompile.New().(*compileJWT.Claims)
	c.ProjectId = projectId
	c.UserId = userId
	c.CompileGroup = ownerFeatures.CompileGroup
	c.Timeout = ownerFeatures.CompileTimeout
	if err := c.EpochItems().Populate(ctx); err != nil {
		return "", err
	}
	return m.jwtCompile.Sign(c)
}

func (m *manager) genJWTSpelling(userId primitive.ObjectID) (string, error) {
	c := m.jwtSpelling.New().(*userIdJWT.Claims)
	c.UserId = userId
	return m.jwtSpelling.Sign(c)
}

func (m *manager) genWSBootstrap(projectId primitive.ObjectID, u user.WithPublicInfo) (types.WSBootstrap, error) {
	c := m.wsBootstrap.New().(*wsBootstrap.Claims)
	c.ProjectId = projectId
	c.User.Id = u.Id
	c.User.Email = u.Email
	c.User.FirstName = u.FirstName
	c.User.LastName = u.LastName

	blob, err := m.wsBootstrap.Sign(c)
	if err != nil {
		return types.WSBootstrap{}, err
	}
	return types.WSBootstrap{
		JWT:       blob,
		ExpiresIn: int64(m.wsBootstrap.ExpiresIn().Seconds()),
	}, nil
}

func (m *manager) LoadEditor(ctx context.Context, request *types.LoadEditorRequest, response *types.LoadEditorResponse) error {
	projectId := request.ProjectId
	userId := request.UserId
	isAnonymous := userId.IsZero()
	if !isAnonymous {
		// Logged-in users must go through the join process first
		request.AnonymousAccessToken = ""
	}

	p, err := m.pm.GetLoadEditorDetails(ctx, projectId, userId)
	if err != nil {
		return err
	}
	authorizationDetails, err := p.GetPrivilegeLevel(
		userId, request.AnonymousAccessToken,
	)
	if err != nil {
		return err
	}

	u := &user.WithLoadEditorInfo{}

	eg, pCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		var ownerFeatures *user.Features
		isOwner := p.OwnerRef == userId
		egInner, pCtxInner := errgroup.WithContext(pCtx)
		if isAnonymous {
			u = defaultUser
		} else {
			egInner.Go(func() error {
				if err2 := m.um.GetUser(pCtxInner, userId, u); err2 != nil {
					return errors.Tag(err2, "cannot get user details")
				}
				if isOwner {
					ownerFeatures = &u.Features
				}
				return nil
			})
		}

		c := m.jwtCompile.New().(*compileJWT.Claims)
		c.ProjectId = projectId
		c.UserId = userId

		egInner.Go(func() error {
			if err2 := c.EpochItems().Populate(ctx); err2 != nil {
				return errors.Tag(err2, "cannot get epochs")
			}
			return nil
		})

		if !isOwner {
			egInner.Go(func() error {
				o := &user.FeaturesField{}
				if err2 := m.um.GetUser(pCtxInner, p.OwnerRef, o); err2 != nil {
					return errors.Tag(err2, "cannot get project owner features")
				}
				ownerFeatures = &o.Features
				return nil
			})
		}

		if err2 := egInner.Wait(); err2 != nil {
			return err2
		}
		c.CompileGroup = ownerFeatures.CompileGroup
		c.Timeout = ownerFeatures.CompileTimeout

		s, err2 := m.jwtCompile.Sign(c)
		if err2 != nil {
			return errors.Tag(err, "cannot get compile jwt")
		}
		response.JWTCompile = s

		response.WSBootstrap, err2 = m.genWSBootstrap(
			projectId, u.WithPublicInfo,
		)
		if err2 != nil {
			return errors.Tag(err, "cannot get wsBootstrap")
		}
		return nil
	})

	if !p.Active {
		eg.Go(func() error {
			if err2 := m.dm.UnArchiveProject(pCtx, projectId); err2 != nil {
				return errors.Tag(err2, "cannot un-archive project")
			}
			if err2 := m.pm.MarkAsActive(pCtx, projectId); err2 != nil {
				return errors.Tag(err2, "cannot mark project as active")
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
			s, err2 := m.genJWTSpelling(userId)
			if err2 != nil {
				return errors.Tag(err, "cannot get spelling jwt")
			}
			response.JWTSpelling = s
			return nil
		})
	}

	if err = eg.Wait(); err != nil {
		return err
	}

	response.Anonymous = isAnonymous
	response.AnonymousAccessToken = request.AnonymousAccessToken
	response.AuthorizationDetails = *authorizationDetails
	response.Project = p.LoadEditorViewPublic
	response.User = *u
	return nil
}
