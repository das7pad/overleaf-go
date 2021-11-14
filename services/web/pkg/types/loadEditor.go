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
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type LoadEditorRequest struct {
	ProjectId            primitive.ObjectID
	UserId               primitive.ObjectID
	AnonymousAccessToken project.AccessToken `form:"anonymousAccessToken"`
}

type LoadEditorResponse struct {
	Anonymous            bool                         `json:"anonymous"`
	AnonymousAccessToken project.AccessToken          `json:"anonymousAccessToken"`
	IsRestrictedUser     project.IsRestrictedUser     `json:"isRestrictedTokenMember"`
	IsTokenMember        project.IsTokenMember        `json:"isTokenMember"`
	JwtProject           string                       `json:"jwtCompile"`
	JWTLoggedInUser      string                       `json:"jwtLoggedInUser"`
	JWTSpelling          string                       `json:"jwtSpelling"`
	PrivilegeLevel       sharedTypes.PrivilegeLevel   `json:"privilegeLevel"`
	Project              project.LoadEditorViewPublic `json:"project"`
	User                 user.WithLoadEditorInfo      `json:"user"`
	WSBootstrap          WSBootstrap                  `json:"wsBootstrap"`
}

type WSBootstrap struct {
	JWT       string `json:"bootstrap"`
	ExpiresIn int64  `json:"expiresIn"`
}
