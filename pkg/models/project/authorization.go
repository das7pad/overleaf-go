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
)

type PrivilegeLevel string
type PublicAccessLevel string
type IsRestrictedUser bool

const (
	PrivilegeLevelOwner        PrivilegeLevel = "owner"
	PrivilegeLevelReadAndWrite PrivilegeLevel = "readAndWrite"
	PrivilegeLevelReadOnly     PrivilegeLevel = "readOnly"

	TokenBasedAccess PublicAccessLevel = "tokenBased"
)

type Refs []primitive.ObjectID

func (r Refs) Contains(userId primitive.ObjectID) bool {
	for _, ref := range r {
		if userId == ref {
			return true
		}
	}
	return false
}

func (p *WithAuthorizationDetails) GetPrivilegeLevel(userId primitive.ObjectID, accessToken AccessToken) (PrivilegeLevel, IsRestrictedUser) {
	if p.OwnerRef == userId {
		return PrivilegeLevelOwner, false
	}
	if userId.IsZero() {
		if p.PublicAccessLevel == TokenBasedAccess && accessToken != "" {
			switch accessToken[0] {
			case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
				// ReadAndWrite tokens start with numeric characters.
				if p.Tokens.ReadAndWrite.EqualsTimingSafe(accessToken) {
					return PrivilegeLevelReadAndWrite, false
				}
			default:
				// ReadOnly tokens are composed of alpha characters only.
				if p.Tokens.ReadOnly.EqualsTimingSafe(accessToken) {
					return PrivilegeLevelReadOnly, true
				}
			}
		}
	} else {
		if p.CollaboratorRefs.Contains(userId) {
			return PrivilegeLevelReadAndWrite, false
		}
		if p.ReadOnlyRefs.Contains(userId) {
			return PrivilegeLevelReadOnly, false
		}
		if p.PublicAccessLevel == TokenBasedAccess {
			if p.TokenAccessReadAndWriteRefs.Contains(userId) {
				return PrivilegeLevelReadAndWrite, false
			}
			if p.TokenAccessReadOnlyRefs.Contains(userId) {
				return PrivilegeLevelReadOnly, true
			}
		}
	}
	return "", false
}
