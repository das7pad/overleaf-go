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
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/projectInvite"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type JoinProjectRequest struct {
	ProjectId            sharedTypes.UUID    `json:"project_id"`
	AnonymousAccessToken project.AccessToken `json:"anonymousAccessToken"`
}

type JoinProjectResponse struct {
	Project          JoinProjectDetails         `json:"project"`
	PrivilegeLevel   sharedTypes.PrivilegeLevel `json:"privilegeLevel"`
	ConnectedClients ConnectedClients           `json:"connectedClients"`
}

type JoinProjectWebApiResponse struct {
	Project          JoinProjectDetails         `json:"project"`
	PrivilegeLevel   sharedTypes.PrivilegeLevel `json:"privilegeLevel"`
	IsRestrictedUser project.IsRestrictedUser   `json:"isRestrictedUser"`
}

type JoinProjectDetails struct {
	project.JoinProjectViewPublic
	project.PublicAccessLevelField
	project.RootDocIdField
	project.TokensField
	project.VersionField
	RootFolder []*project.Folder            `json:"rootFolder"`
	Invites    []projectInvite.WithoutToken `json:"invites"`
	Members    []user.AsProjectMember       `json:"members"`
	Owner      user.WithPublicInfo          `json:"owner"`
}
