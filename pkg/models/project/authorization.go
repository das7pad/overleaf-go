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

package project

import (
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type AccessSource string
type PublicAccessLevel string
type IsRestrictedUser bool
type IsTokenMember bool

const (
	AccessSourceOwner  AccessSource = "owner"
	AccessSourceToken  AccessSource = "token"
	AccessSourceInvite AccessSource = "invite"

	TokenBasedAccess PublicAccessLevel = "tokenBased"
)

type AuthorizationDetails struct {
	Epoch          int64                      `json:"e,omitempty"`
	PrivilegeLevel sharedTypes.PrivilegeLevel `json:"l"`
	IsTokenMember  IsTokenMember              `json:"tm,omitempty"`
	AccessSource   AccessSource               `json:"-"`
}

func (a *AuthorizationDetails) IsRestrictedUser() IsRestrictedUser {
	return IsRestrictedUser(
		a.IsTokenMember && a.PrivilegeLevel == sharedTypes.PrivilegeLevelReadOnly,
	)
}

type Refs []primitive.ObjectID

func (r Refs) Contains(userId primitive.ObjectID) bool {
	for _, ref := range r {
		if userId == ref {
			return true
		}
	}
	return false
}

func (p *ForAuthorizationDetails) GetPrivilegeLevelAnonymous(accessToken AccessToken) (*AuthorizationDetails, error) {
	if p.PublicAccessLevel == TokenBasedAccess && accessToken != "" {
		switch accessToken[0] {
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			// ReadAndWrite tokens start with numeric characters.
			if p.Tokens.ReadAndWrite.EqualsTimingSafe(accessToken) {
				return &AuthorizationDetails{
					Epoch:          p.Epoch,
					AccessSource:   AccessSourceToken,
					PrivilegeLevel: sharedTypes.PrivilegeLevelReadAndWrite,
					IsTokenMember:  true,
				}, nil
			}
		default:
			// ReadOnly tokens are composed of alpha characters only.
			if p.Tokens.ReadOnly.EqualsTimingSafe(accessToken) {
				return &AuthorizationDetails{
					Epoch:          p.Epoch,
					AccessSource:   AccessSourceToken,
					PrivilegeLevel: sharedTypes.PrivilegeLevelReadOnly,
					IsTokenMember:  true,
				}, nil
			}
		}
	}
	return &AuthorizationDetails{Epoch: p.Epoch}, &errors.NotAuthorizedError{}
}

func (p *ForAuthorizationDetails) GetPrivilegeLevelAuthenticated(userId primitive.ObjectID) (*AuthorizationDetails, error) {
	if p.OwnerRef == userId {
		return &AuthorizationDetails{
			Epoch:          p.Epoch,
			AccessSource:   AccessSourceOwner,
			PrivilegeLevel: sharedTypes.PrivilegeLevelOwner,
			IsTokenMember:  false,
		}, nil
	}
	if p.CollaboratorRefs.Contains(userId) {
		return &AuthorizationDetails{
			Epoch:          p.Epoch,
			AccessSource:   AccessSourceInvite,
			PrivilegeLevel: sharedTypes.PrivilegeLevelReadAndWrite,
			IsTokenMember:  false,
		}, nil
	}
	if p.ReadOnlyRefs.Contains(userId) {
		return &AuthorizationDetails{
			Epoch:          p.Epoch,
			AccessSource:   AccessSourceInvite,
			PrivilegeLevel: sharedTypes.PrivilegeLevelReadOnly,
			IsTokenMember:  false,
		}, nil
	}
	if p.PublicAccessLevel == TokenBasedAccess {
		if p.TokenAccessReadAndWriteRefs.Contains(userId) {
			return &AuthorizationDetails{
				Epoch:          p.Epoch,
				AccessSource:   AccessSourceToken,
				PrivilegeLevel: sharedTypes.PrivilegeLevelReadAndWrite,
				IsTokenMember:  true,
			}, nil
		}
		if p.TokenAccessReadOnlyRefs.Contains(userId) {
			return &AuthorizationDetails{
				Epoch:          p.Epoch,
				AccessSource:   AccessSourceToken,
				PrivilegeLevel: sharedTypes.PrivilegeLevelReadOnly,
				IsTokenMember:  true,
			}, nil
		}
	}
	return &AuthorizationDetails{Epoch: p.Epoch}, &errors.NotAuthorizedError{}
}

func (p *ForAuthorizationDetails) GetPrivilegeLevel(userId primitive.ObjectID, accessToken AccessToken) (*AuthorizationDetails, error) {
	if userId.IsZero() {
		return p.GetPrivilegeLevelAnonymous(accessToken)
	} else {
		return p.GetPrivilegeLevelAuthenticated(userId)
	}
}

type TokenAccessResult struct {
	ProjectId primitive.ObjectID
	Fresh     *AuthorizationDetails
	Existing  *AuthorizationDetails
}

func (r *TokenAccessResult) ShouldGrantHigherAccess() bool {
	if r.Existing == nil {
		return true
	}
	return r.Fresh.PrivilegeLevel.IsHigherThan(r.Existing.PrivilegeLevel)
}
