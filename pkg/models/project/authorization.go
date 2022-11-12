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

package project

import (
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type AccessSource string

type PublicAccessLevel string

func (l PublicAccessLevel) Validate() error {
	switch l {
	case PrivateAccess:
	case TokenBasedAccess:
	default:
		return &errors.ValidationError{Msg: "unknown PublicAccessLevel"}
	}
	return nil
}

type IsRestrictedUser bool

const (
	AccessSourceOwner  AccessSource = "owner"
	AccessSourceToken  AccessSource = "token"
	AccessSourceInvite AccessSource = "invite"

	TokenBasedAccess PublicAccessLevel = "tokenBased"
	PrivateAccess    PublicAccessLevel = "private"
)

type AuthorizationDetails struct {
	Epoch          int64                      `json:"e"`
	PrivilegeLevel sharedTypes.PrivilegeLevel `json:"l"`
	AccessSource   AccessSource               `json:"s"`
}

func (a *AuthorizationDetails) IsTokenMember() bool {
	return a.AccessSource == AccessSourceToken
}

func (a *AuthorizationDetails) IsRestrictedUser() IsRestrictedUser {
	return IsRestrictedUser(
		a.IsTokenMember() &&
			a.PrivilegeLevel == sharedTypes.PrivilegeLevelReadOnly,
	)
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
				}, nil
			}
		default:
			// ReadOnly tokens are composed of alpha characters only.
			if p.Tokens.ReadOnly.EqualsTimingSafe(accessToken) {
				return &AuthorizationDetails{
					Epoch:          p.Epoch,
					AccessSource:   AccessSourceToken,
					PrivilegeLevel: sharedTypes.PrivilegeLevelReadOnly,
				}, nil
			}
		}
	}
	return &AuthorizationDetails{Epoch: p.Epoch}, &errors.NotAuthorizedError{}
}

func (p *ForAuthorizationDetails) GetPrivilegeLevelAuthenticated() (*AuthorizationDetails, error) {
	switch {
	case
		p.AccessSource == AccessSourceOwner,
		p.AccessSource == AccessSourceInvite,
		p.AccessSource == AccessSourceToken &&
			// Access details remain as is in db when disabling link sharing,
			//  we need to check in app code for validity.
			p.PublicAccessLevel == TokenBasedAccess:
		return &AuthorizationDetails{
			Epoch:          p.Epoch,
			AccessSource:   p.Member.AccessSource,
			PrivilegeLevel: p.Member.PrivilegeLevel,
		}, nil
	default:
		return &AuthorizationDetails{
			Epoch: p.Epoch,
		}, &errors.NotAuthorizedError{}
	}
}

func (p *ForAuthorizationDetails) GetPrivilegeLevel(userId sharedTypes.UUID, accessToken AccessToken) (*AuthorizationDetails, error) {
	if userId.IsZero() {
		return p.GetPrivilegeLevelAnonymous(accessToken)
	} else {
		return p.GetPrivilegeLevelAuthenticated()
	}
}

type TokenAccessDetails struct {
	ProjectId sharedTypes.UUID
	Fresh     *AuthorizationDetails
	Existing  *AuthorizationDetails
}

func (r *TokenAccessDetails) ShouldGrantHigherAccess() bool {
	if r.Existing == nil {
		return true
	}
	return r.Fresh.PrivilegeLevel.IsHigherThan(r.Existing.PrivilegeLevel)
}
