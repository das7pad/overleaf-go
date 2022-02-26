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

package types

import (
	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/session"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type LeaveProjectRequest struct {
	Session   *session.Session `json:"-"`
	ProjectId edgedb.UUID      `json:"-"`
}

type ListProjectMembersRequest struct {
	ProjectId edgedb.UUID `json:"-"`
}

type ListProjectMembersResponse struct {
	Members []user.AsProjectMember `json:"members"`
}

type RemoveProjectMemberRequest struct {
	ProjectId edgedb.UUID `json:"-"`
	Epoch     int64       `json:"-"`
	UserId    edgedb.UUID `json:"-"`
}

type SetMemberPrivilegeLevelInProjectRequest struct {
	ProjectId      edgedb.UUID                `json:"-"`
	UserId         edgedb.UUID                `json:"-"`
	PrivilegeLevel sharedTypes.PrivilegeLevel `json:"privilegeLevel"`
}

type TransferProjectOwnershipRequest struct {
	ProjectId       edgedb.UUID `json:"-"`
	PreviousOwnerId edgedb.UUID `json:"-"`
	NewOwnerId      edgedb.UUID `json:"user_id"`
}
