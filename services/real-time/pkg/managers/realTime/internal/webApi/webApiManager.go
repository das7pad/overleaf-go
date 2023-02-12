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

package webApi

import (
	"context"

	"github.com/jackc/pgx/v4/pgxpool"

	"github.com/das7pad/overleaf-go/pkg/jwt/projectJWT"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/projectInvite"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

type Manager interface {
	BootstrapWS(ctx context.Context, claims projectJWT.Claims) (types.User, *types.JoinProjectWebApiResponse, error)
}

func New(db *pgxpool.Pool) Manager {
	return &manager{
		pm: project.New(db),
	}
}

type manager struct {
	pm project.Manager
}

func (m *manager) BootstrapWS(ctx context.Context, claims projectJWT.Claims) (types.User, *types.JoinProjectWebApiResponse, error) {
	d, err := m.pm.GetBootstrapWSDetails(
		ctx, claims.ProjectId, claims.UserId, claims.Epoch, claims.EpochUser,
	)
	if err != nil {
		return types.User{}, nil, err
	}

	p := &d.Project
	authorizationDetails := claims.AuthorizationDetails

	var tokens project.Tokens
	if authorizationDetails.PrivilegeLevel == sharedTypes.PrivilegeLevelOwner {
		// Expose all tokens to the owner
		tokens = p.Tokens
	} else if authorizationDetails.IsRestrictedUser() {
		// Expose read-only token to read-only token user.
		tokens = project.Tokens{ReadOnly: p.Tokens.ReadOnly}
	}

	// Hide owner details from token users
	owner := user.WithPublicInfo{}
	owner.Id = p.OwnerId
	if authorizationDetails.AccessSource != project.AccessSourceToken {
		owner = d.Owner
	}

	// Populate fake feature flags
	p.OwnerFeatures.Collaborators = -1
	p.OwnerFeatures.TrackChangesVisible = false
	p.OwnerFeatures.TrackChanges = false
	p.OwnerFeatures.Versioning = true

	details := types.JoinProjectDetails{
		JoinProjectViewPublic:  p.JoinProjectViewPublic,
		Invites:                make([]projectInvite.WithoutToken, 0),
		Members:                make([]user.AsProjectMember, 0),
		TokensField:            project.TokensField{Tokens: tokens},
		PublicAccessLevelField: p.PublicAccessLevelField,
		Owner:                  owner,
		RootDocIdField: project.RootDocIdField{
			RootDocId: p.RootDoc.Id,
		},
		VersionField: p.VersionField,
		RootFolder:   []*project.Folder{p.GetRootFolder()},
	}

	return types.User{
			Id:        claims.UserId,
			FirstName: d.User.FirstName,
			LastName:  d.User.LastName,
			Email:     d.User.Email,
		}, &types.JoinProjectWebApiResponse{
			Project:          details,
			PrivilegeLevel:   authorizationDetails.PrivilegeLevel,
			IsRestrictedUser: authorizationDetails.IsRestrictedUser(),
		}, nil
}
