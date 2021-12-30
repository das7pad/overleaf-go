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

package webApi

import (
	"context"

	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/doc"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/projectInvite"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/docstore/pkg/managers/docstore"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

type monolithManager struct {
	dm  docstore.Manager
	pim projectInvite.Manager
	pm  project.Manager
	um  user.Manager
}

const self = "self"

func (m *monolithManager) JoinProject(ctx context.Context, client *types.Client, request *types.JoinProjectRequest) (*types.JoinProjectWebApiResponse, string, error) {
	userId := client.User.Id
	p, err := m.pm.GetJoinProjectDetails(ctx, request.ProjectId, userId)
	if err != nil {
		return nil, self, errors.Tag(err, "cannot get project")
	}

	authorizationDetails, err := p.GetPrivilegeLevel(
		userId, request.AnonymousAccessToken,
	)
	if err != nil {
		return nil, self, err
	}

	var deletedDocs []doc.Name
	owner := &user.WithPublicInfoAndFeatures{}
	members := make([]user.AsProjectMember, 0)
	invites := make([]*projectInvite.WithoutToken, 0)

	eg, pCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		var err2 error
		deletedDocs, err2 = m.dm.PeakDeletedDocNames(pCtx, request.ProjectId)
		if err2 != nil {
			return errors.Tag(err, "cannot get deleted doc names")
		}
		return nil
	})
	eg.Go(func() error {
		if err2 := m.um.GetUser(pCtx, p.OwnerRef, owner); err2 != nil {
			return errors.Tag(err2, "cannot get project owner")
		}
		return nil
	})
	if !authorizationDetails.IsRestrictedUser() {
		eg.Go(func() error {
			if n := len(p.CollaboratorRefs) + len(p.ReadOnlyRefs); n == 0 {
				return nil
			}
			var err2 error
			members, err2 = m.um.GetProjectMembers(
				pCtx, p.ReadOnlyRefs, p.CollaboratorRefs,
			)
			if err2 != nil {
				return errors.Tag(err2, "cannot get members")
			}
			return nil
		})
		eg.Go(func() error {
			var err2 error
			invites, err2 = m.pim.GetAllForProject(pCtx, request.ProjectId)
			if err2 != nil {
				return errors.Tag(err2, "cannot get project invites")
			}
			return nil
		})
	}
	if err = eg.Wait(); err != nil {
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

	// Hide owner details for restricted users
	if authorizationDetails.IsRestrictedUser() {
		owner.WithPublicInfo = user.WithPublicInfo{
			IdField: user.IdField{Id: p.OwnerRef},
		}
	}

	// Populate fake feature flag
	owner.Features.TrackChangesVisible = true

	details := types.JoinProjectDetails{
		DeletedDocs:            deletedDocs,
		Features:               owner.Features,
		JoinProjectViewPublic:  p.JoinProjectViewPublic,
		Members:                members,
		Owner:                  owner.WithPublicInfo,
		TokensField:            project.TokensField{Tokens: tokens},
		PublicAccessLevelField: p.PublicAccessLevelField,
		Invites:                invites,
	}

	return &types.JoinProjectWebApiResponse{
		Project:          details,
		PrivilegeLevel:   authorizationDetails.PrivilegeLevel,
		IsRestrictedUser: authorizationDetails.IsRestrictedUser(),
	}, self, nil
}
