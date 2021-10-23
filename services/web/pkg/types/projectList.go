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
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/tag"
	"github.com/das7pad/overleaf-go/pkg/models/user"
)

type ProjectListRequest struct {
	UserId primitive.ObjectID
}

type ProjectListProjectView struct {
	Id                  primitive.ObjectID        `json:"id"`
	Name                string                    `json:"name"`
	LastUpdatedAt       time.Time                 `json:"lastUpdated"`
	LastUpdatedByUserId primitive.ObjectID        `json:"-"`
	LastUpdatedBy       *user.WithPublicInfo      `json:"lastUpdatedBy"`
	PublicAccessLevel   project.PublicAccessLevel `json:"publicAccessLevel"`
	AccessLevel         project.PrivilegeLevel    `json:"accessLevel"`
	AccessSource        project.AccessSource      `json:"source"`
	Archived            bool                      `json:"archived"`
	Trashed             bool                      `json:"trashed"`
	OwnerRef            primitive.ObjectID        `json:"owner_ref"`
	Owner               *user.WithPublicInfo      `json:"owner"`
}

type ProjectListResponse struct {
	Projects         []*ProjectListProjectView          `json:"projects"`
	JWTLoggedInUser  string                             `json:"jwtLoggedInUser"`
	JWTNotifications string                             `json:"jwtNotifications"`
	Tags             []tag.Full                         `json:"tags"`
	UserEmails       []user.EmailDetailsWithDefaultFlag `json:"userEmails"`
}
