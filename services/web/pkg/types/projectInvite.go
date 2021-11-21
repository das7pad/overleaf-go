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

	"github.com/das7pad/overleaf-go/pkg/asyncForm"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/projectInvite"
	"github.com/das7pad/overleaf-go/pkg/session"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type AcceptProjectInviteRequest struct {
	Session   *session.Session    `json:"-"`
	ProjectId primitive.ObjectID  `json:"-"`
	Token     projectInvite.Token `json:"-"`
}

type AcceptProjectInviteResponse = asyncForm.Response

type CreateProjectInviteRequest struct {
	ProjectId      primitive.ObjectID         `json:"-"`
	SenderUserId   primitive.ObjectID         `json:"-"`
	Email          sharedTypes.Email          `json:"email"`
	PrivilegeLevel sharedTypes.PrivilegeLevel `json:"privileges"`
}

func (r *CreateProjectInviteRequest) Preprocess() {
	r.Email = r.Email.Normalize()
}

func (r *CreateProjectInviteRequest) Validate() error {
	if err := r.Email.Validate(); err != nil {
		return err
	}
	if err := r.PrivilegeLevel.Validate(); err != nil {
		return err
	}
	if r.PrivilegeLevel == sharedTypes.PrivilegeLevelOwner {
		return &errors.ValidationError{
			Msg: "use project ownership transfer instead",
		}
	}
	return nil
}

type ResendProjectInviteRequest struct {
	ProjectId primitive.ObjectID `json:"-"`
	InviteId  primitive.ObjectID `json:"-"`
}

type RevokeProjectInviteRequest struct {
	ProjectId primitive.ObjectID `json:"-"`
	InviteId  primitive.ObjectID `json:"-"`
}

type ListProjectInvitesRequest struct {
	ProjectId primitive.ObjectID `json:"-"`
}

type ListProjectInvitesResponse struct {
	Invites []*projectInvite.WithoutToken `json:"invites"`
}
