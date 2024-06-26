// Golang port of Overleaf
// Copyright (C) 2021-2024 Jakob Ackermann <das7pad@outlook.com>
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

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) GetProjectJWT(ctx context.Context, request *types.GetProjectJWTRequest, response *types.GetProjectJWTResponse) error {
	projectId := request.ProjectId
	userId := request.Session.User.Id

	accessToken := request.Session.GetAnonTokenAccess(projectId)
	p, userEpoch, err := m.pm.GetForProjectJWT(
		ctx, projectId, userId, accessToken,
	)
	if err != nil {
		return errors.Tag(err, "get project/user details")
	}

	authorizationDetails, err := p.GetPrivilegeLevel(userId, accessToken)
	if err != nil {
		return err
	}

	if userId.IsZero() {
		userEpoch = user.AnonymousUserEpoch
	}

	c := m.jwtProject.New()
	c.ProjectId = projectId
	c.UserId = userId
	c.CompileGroup = p.OwnerFeatures.CompileGroup
	c.Timeout = p.OwnerFeatures.CompileTimeout
	c.Editable = p.Editable
	c.EpochUser = userEpoch
	c.AuthorizationDetails = *authorizationDetails

	s, err := m.jwtProject.SetExpiryAndSign(c)
	if err != nil {
		return errors.Tag(err, "sign jwt")
	}
	*response = types.GetProjectJWTResponse(s)
	return nil
}
