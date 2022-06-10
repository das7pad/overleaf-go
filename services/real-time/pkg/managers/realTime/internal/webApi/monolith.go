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

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/projectInvite"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

type monolithManager struct {
	pm project.Manager
}

const self = "self"

func (m *monolithManager) JoinProject(ctx context.Context, client *types.Client, request *types.JoinProjectRequest) (*types.JoinProjectWebApiResponse, string, error) {
	userId := client.User.Id
	d, err := m.pm.GetJoinProjectDetails(
		ctx, request.ProjectId, userId, request.AnonymousAccessToken,
	)
	if err != nil {
		return nil, self, errors.Tag(err, "cannot get project")
	}
	p := &d.Project

	authorizationDetails, err := p.GetPrivilegeLevel(
		userId, request.AnonymousAccessToken,
	)
	if err != nil {
		return nil, self, err
	}

	// Expose a subset of link sharing tokens.
	var tokens project.Tokens
	switch authorizationDetails.PrivilegeLevel {
	case sharedTypes.PrivilegeLevelOwner:
		tokens = p.Tokens
	case sharedTypes.PrivilegeLevelReadAndWrite:
		tokens = project.Tokens{ReadAndWrite: p.Tokens.ReadAndWrite}
	case sharedTypes.PrivilegeLevelReadOnly:
		tokens = project.Tokens{ReadOnly: p.Tokens.ReadOnly}
	}

	members := make([]user.AsProjectMember, 0)
	owner := user.WithPublicInfo{}
	owner.Id = p.OwnerId

	// Hide user details for restricted users
	if authorizationDetails.IsRestrictedUser() {
		d.Project.Invites = make([]projectInvite.WithoutToken, 0)
	} else {
		members = p.GetProjectMembers()
		// TODO: populate owner?
	}

	// Populate fake feature flags
	p.OwnerFeatures.Collaborators = -1
	p.OwnerFeatures.TrackChangesVisible = false
	p.OwnerFeatures.TrackChanges = false
	p.OwnerFeatures.Versioning = true

	details := types.JoinProjectDetails{
		JoinProjectViewPublic:  p.JoinProjectViewPublic,
		Members:                members,
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
	}, self, nil
}
