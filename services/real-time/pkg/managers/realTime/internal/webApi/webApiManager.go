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
	"database/sql"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/projectInvite"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

type Manager interface {
	JoinProject(ctx context.Context, client *types.Client, request *types.JoinProjectRequest) (*types.JoinProjectWebApiResponse, error)
}

func New(db *sql.DB) Manager {
	return &manager{
		pm: project.New(db),
	}
}

type manager struct {
	pm project.Manager
}

func (m *manager) JoinProject(ctx context.Context, client *types.Client, request *types.JoinProjectRequest) (*types.JoinProjectWebApiResponse, error) {
	userId := client.User.Id
	d, err := m.pm.GetJoinProjectDetails(
		ctx, request.ProjectId, userId, request.AnonymousAccessToken,
	)
	if err != nil {
		return nil, errors.Tag(err, "cannot get project")
	}
	p := &d.Project

	authorizationDetails, err := p.GetPrivilegeLevel(
		userId, request.AnonymousAccessToken,
	)
	if err != nil {
		return nil, err
	}

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
	if authorizationDetails.AccessSource != "token" {
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

	return &types.JoinProjectWebApiResponse{
		Project:          details,
		PrivilegeLevel:   authorizationDetails.PrivilegeLevel,
		IsRestrictedUser: authorizationDetails.IsRestrictedUser(),
	}, nil
}
