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

	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/user"
)

type JoinProjectRequest struct {
	ProjectId            primitive.ObjectID  `json:"project_id"`
	AnonymousAccessToken project.AccessToken `json:"anonymousAccessToken"`
}

type JoinProjectResponse struct {
	Project          JoinProjectDetails     `json:"project"`
	PrivilegeLevel   project.PrivilegeLevel `json:"privilegeLevel"`
	ConnectedClients ConnectedClients       `json:"connectedClients"`
}

type JoinProjectWebApiResponse struct {
	Project          JoinProjectDetails       `json:"project"`
	PrivilegeLevel   project.PrivilegeLevel   `json:"privilegeLevel"`
	IsRestrictedUser project.IsRestrictedUser `json:"isRestrictedUser"`
}

type JoinProjectDetails struct {
	project.JoinProjectViewPublic
	project.PublicAccessLevelField
	project.TokensField
	Features user.Features         `json:"features"`
	Owner    user.WithPublicInfo   `json:"owner"`
	Members  []user.WithPublicInfo `json:"members"`
	Invites  []interface{}         `json:"invites"`
}
