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

package projectInvite

import (
	"time"

	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type CreatedAtField struct {
	CreatedAt time.Time `json:"createdAt"`
}

type EmailField struct {
	Email sharedTypes.Email `json:"email"`
}

type ExpiresAtField struct {
	Expires time.Time `json:"expires"`
}

type IdField struct {
	Id sharedTypes.UUID `json:"_id"`
}

type PrivilegeLevelField struct {
	PrivilegeLevel sharedTypes.PrivilegeLevel `json:"privileges"`
}

type ProjectField struct {
	ProjectId sharedTypes.UUID `json:"-"`
}

type SendingUserField struct {
	SendingUser user.WithPublicInfo `json:"-"`
}

type TokenField struct {
	Token Token `json:"-"`
}
