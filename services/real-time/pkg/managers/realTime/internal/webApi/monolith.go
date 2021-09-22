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
	"crypto/subtle"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

type monolithManager struct {
	pm project.Manager
	um user.Manager
}

const self = "self"

func tokenMatches(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func isInRefs(refs []primitive.ObjectID, userId primitive.ObjectID) bool {
	for _, ref := range refs {
		if userId == ref {
			return true
		}
	}
	return false
}

func getPrivilegeLevel(p *project.JoinProjectViewPrivate, userId primitive.ObjectID, token string) (types.PrivilegeLevel, types.IsRestrictedUser) {
	if p.OwnerRef == userId {
		return types.PrivilegeLevelOwner, false
	}
	if !userId.IsZero() {
		if isInRefs(p.CollaboratorRefs, userId) {
			return types.PrivilegeLevelReadAndWrite, false
		}
		if isInRefs(p.ReadOnlyRefs, userId) {
			return types.PrivilegeLevelReadOnly, false
		}
	}
	if token == "" || p.PublicAccessLevel != "tokenBased" {
		return "", false
	}
	switch token[0] {
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		// ReadAndWrite tokens start with a sequence of numeric characters.
		if tokenMatches(token, p.Tokens.ReadAndWrite) {
			return types.PrivilegeLevelReadAndWrite, false
		}
	default:
		// ReadOnly tokens are composed of alpha characters only.
		if tokenMatches(token, p.Tokens.ReadOnly) {
			return types.PrivilegeLevelReadOnly, true
		}
	}
	return "", false
}

func (m *monolithManager) JoinProject(ctx context.Context, client *types.Client, request *types.JoinProjectRequest) (*types.JoinProjectWebApiResponse, string, error) {
	userId := client.User.Id
	p, err := m.pm.GetJoinProjectDetails(ctx, request.ProjectId, userId)
	if err != nil {
		return nil, self, errors.Tag(err, "cannot get project")
	}

	privilegeLevel, isRestrictedUser := getPrivilegeLevel(
		p, userId, request.AnonymousAccessToken,
	)
	if privilegeLevel == "" {
		return nil, self, &errors.NotAuthorizedError{}
	}

	var owner *user.WithPublicInfoAndFeatures
	members := make([]user.WithPublicInfo, 0)

	eg, pCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		var err2 error
		owner, err2 = m.um.GetUserWithPublicInfoAndFeatures(pCtx, p.OwnerRef)
		if err2 != nil {
			return errors.Tag(err2, "cannot get project owner")
		}
		return nil
	})
	if !isRestrictedUser {
		eg.Go(func() error {
			n := len(p.CollaboratorRefs) + len(p.ReadOnlyRefs)
			if n == 0 {
				return nil
			}
			allIds := make([]primitive.ObjectID, n)
			copy(allIds, p.CollaboratorRefs)
			copy(allIds[len(p.CollaboratorRefs):], p.ReadOnlyRefs)
			var err2 error
			members, err2 = m.um.GetUsersWithPublicInfo(pCtx, allIds)
			if err2 != nil {
				return errors.Tag(err2, "cannot get members")
			}
			return nil
		})
	}
	if err = eg.Wait(); err != nil {
		return nil, self, err
	}

	// Hide some link sharing tokens.
	switch privilegeLevel {
	case types.PrivilegeLevelOwner:
		// The owner can see them all.
	case types.PrivilegeLevelReadAndWrite:
		p.Tokens.ReadOnly = ""
	case types.PrivilegeLevelReadOnly:
		p.Tokens.ReadAndWritePrefix = ""
		p.Tokens.ReadAndWrite = ""
	}

	// Hide owner details for restricted users
	if isRestrictedUser {
		owner.WithPublicInfo = user.WithPublicInfo{
			IdField: user.IdField{Id: p.OwnerRef},
		}
	}

	// Populate fake feature flag
	owner.Features.TrackChangesVisible = true

	details := types.JoinProjectDetails{
		Features:              owner.Features,
		JoinProjectViewPublic: p.JoinProjectViewPublic,
		Members:               members,
		Owner:                 owner.WithPublicInfo,
	}

	return &types.JoinProjectWebApiResponse{
		Project:          details,
		PrivilegeLevel:   privilegeLevel,
		IsRestrictedUser: isRestrictedUser,
	}, self, nil
}
