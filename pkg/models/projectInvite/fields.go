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

package projectInvite

import (
	"time"

	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type CreatedAtField struct {
	CreatedAt time.Time `json:"createdAt" edgedb:"created_at"`
}

type EmailField struct {
	Email sharedTypes.Email `json:"email" edgedb:"email"`
}

type ExpiresAtField struct {
	Expires time.Time `json:"expires" edgedb:"expires_at"`
}

type IdField struct {
	Id edgedb.UUID `json:"_id" edgedb:"id"`
}

type PrivilegeLevelField struct {
	PrivilegeLevel sharedTypes.PrivilegeLevel `json:"privileges" edgedb:"privilege_level"`
}

type ProjectField struct {
	ProjectId edgedb.UUID `json:"-" edgedb:"id"`
}

type SendingUserField struct {
	SendingUser user.WithPublicInfo `json:"-" edgedb:"sending_user"`
}

type TokenField struct {
	Token Token `json:"-" edgedb:"token"`
}
