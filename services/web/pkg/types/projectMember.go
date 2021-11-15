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
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/session"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type LeaveProjectRequest struct {
	Session   *session.Session   `json:"-"`
	ProjectId primitive.ObjectID `json:"-"`
}

type ListProjectMembersRequest struct {
	ProjectId primitive.ObjectID `json:"-"`
}

type ListProjectMembersResponse struct {
	Members []user.AsProjectMember `json:"members"`
}

type RemoveProjectMemberRequest struct {
	ProjectId primitive.ObjectID `json:"-"`
	UserId    primitive.ObjectID `json:"-"`
}

type SetMemberPrivilegeLevelInProjectRequest struct {
	ProjectId      primitive.ObjectID         `json:"-"`
	UserId         primitive.ObjectID         `json:"-"`
	PrivilegeLevel sharedTypes.PrivilegeLevel `json:"privilegeLevel"`
}
