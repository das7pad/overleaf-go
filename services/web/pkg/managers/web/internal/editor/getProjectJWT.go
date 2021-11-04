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

	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/jwt/projectJWT"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) GetProjectJWT(ctx context.Context, request *types.GetProjectJWTRequest, response *types.GetProjectJWTResponse) error {
	projectId := request.ProjectId
	userId := request.Session.User.Id

	p := &project.ForAuthorizationDetails{}
	if err := m.pm.GetProject(ctx, projectId, p); err != nil {
		return errors.Tag(err, "cannot get project from mongo")
	}

	accessToken := request.Session.AnonTokenAccess[projectId.Hex()]
	authorizationDetails, err := p.GetPrivilegeLevel(userId, accessToken)
	if err != nil {
		return err
	}

	var ownerFeatures user.Features
	var userEpoch user.EpochField
	eg, pCtx := errgroup.WithContext(ctx)
	if p.OwnerRef == userId {
		eg.Go(func() error {
			u := &user.WithEpochAndFeatures{}
			if err2 := m.um.GetUser(pCtx, userId, u); err2 != nil {
				return errors.Tag(err2, "cannot get epoch/owner features")
			}
			ownerFeatures = u.Features
			userEpoch = u.EpochField
			return nil
		})
	} else {
		eg.Go(func() error {
			u := &user.FeaturesField{}
			if err2 := m.um.GetUser(pCtx, p.OwnerRef, u); err2 != nil {
				return errors.Tag(err2, "cannot get owner features")
			}
			ownerFeatures = u.Features
			return nil
		})
		eg.Go(func() error {
			if err2 := m.um.GetUser(pCtx, userId, &userEpoch); err2 != nil {
				return errors.Tag(err2, "cannot get user epoch")
			}
			return nil
		})
	}
	if err = eg.Wait(); err != nil {
		return err
	}

	c := m.jwtProject.New().(*projectJWT.Claims)
	c.ProjectId = projectId
	c.UserId = userId
	c.CompileGroup = ownerFeatures.CompileGroup
	c.Timeout = ownerFeatures.CompileTimeout
	c.EpochUser = userEpoch.Epoch
	c.AuthorizationDetails = *authorizationDetails

	s, err := m.jwtProject.SetExpiryAndSign(c)
	if err != nil {
		return errors.Tag(err, "cannot sign jwt")
	}
	*response = types.GetProjectJWTResponse(s)
	return nil
}
