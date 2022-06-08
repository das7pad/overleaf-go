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

package types

import (
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/session"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type LeaveProjectRequest struct {
	Session   *session.Session `json:"-"`
	ProjectId sharedTypes.UUID `json:"-"`
}

type ListProjectMembersRequest struct {
	ProjectId sharedTypes.UUID `json:"-"`
}

type ListProjectMembersResponse struct {
	Members []user.AsProjectMember `json:"members"`
}

type RemoveProjectMemberRequest struct {
	ProjectId sharedTypes.UUID `json:"-"`
	ActorId   sharedTypes.UUID `json:"-"`
	UserId    sharedTypes.UUID `json:"-"`
}

type SetMemberPrivilegeLevelInProjectRequest struct {
	ProjectId      sharedTypes.UUID           `json:"-"`
	UserId         sharedTypes.UUID           `json:"-"`
	PrivilegeLevel sharedTypes.PrivilegeLevel `json:"privilegeLevel"`
}

type TransferProjectOwnershipRequest struct {
	ProjectId       sharedTypes.UUID `json:"-"`
	PreviousOwnerId sharedTypes.UUID `json:"-"`
	NewOwnerId      sharedTypes.UUID `json:"user_id"`
}
