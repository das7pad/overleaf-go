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

package templates

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/projectInvite"
	"github.com/das7pad/overleaf-go/pkg/models/tag"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type ProjectTokenAccessData struct {
	AngularLayoutData

	PostURL *sharedTypes.URL
}

func (d *ProjectTokenAccessData) Meta() []metaEntry {
	out := d.AngularLayoutData.Meta()
	out = append(out, metaEntry{
		Name:    "ol-postURL",
		Content: d.PostURL.String(),
		Type:    stringContentType,
	})
	return out
}

func (d *ProjectTokenAccessData) Render() (string, error) {
	return render("project/tokenAccess.gohtml", 30*1024, d)
}

type ProjectViewInviteData struct {
	MarketingLayoutData

	SharedProjectData SharedProjectData
	ProjectId         primitive.ObjectID
	Token             projectInvite.Token
	Valid             bool
}

func (d *ProjectViewInviteData) Render() (string, error) {
	return render("project/viewInvite.gohtml", 30*1024, d)
}

type ProjectListProjectView struct {
	Id                  primitive.ObjectID         `json:"id"`
	Name                project.Name               `json:"name"`
	LastUpdatedAt       time.Time                  `json:"lastUpdated"`
	LastUpdatedByUserId primitive.ObjectID         `json:"-"`
	LastUpdatedBy       *user.WithPublicInfo       `json:"lastUpdatedBy"`
	PublicAccessLevel   project.PublicAccessLevel  `json:"publicAccessLevel"`
	AccessLevel         sharedTypes.PrivilegeLevel `json:"accessLevel"`
	AccessSource        project.AccessSource       `json:"source"`
	Archived            bool                       `json:"archived"`
	Trashed             bool                       `json:"trashed"`
	OwnerRef            primitive.ObjectID         `json:"owner_ref"`
	Owner               *user.WithPublicInfo       `json:"owner"`
}

type ProjectListData struct {
	AngularLayoutData

	Projects         []*ProjectListProjectView
	Tags             []tag.Full
	JWTLoggedInUser  string
	UserEmails       []user.EmailDetailsWithDefaultFlag
	SuggestedLngCode string
}

func (d *ProjectListData) Meta() []metaEntry {
	out := d.AngularLayoutData.Meta()
	out = append(out, metaEntry{
		Name:    "ol-projects",
		Content: d.Projects,
		Type:    jsonContentType,
	})
	out = append(out, metaEntry{
		Name:    "ol-tags",
		Content: d.Tags,
		Type:    jsonContentType,
	})
	out = append(out, metaEntry{
		Name:    "ol-userEmails",
		Content: d.UserEmails,
		Type:    jsonContentType,
	})
	out = append(out, metaEntry{
		Name:    "ol-jwtLoggedInUser",
		Content: d.JWTLoggedInUser,
		Type:    stringContentType,
	})
	return out
}

func (d *ProjectListData) Render() (string, error) {
	return render("project/list.gohtml", 200*1024, d)
}
