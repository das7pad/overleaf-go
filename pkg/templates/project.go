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
	clsiTypes "github.com/das7pad/overleaf-go/services/clsi/pkg/types"
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

func (d *ProjectTokenAccessData) Render() ([]byte, error) {
	return render("project/tokenAccess.gohtml", 6*1024, d)
}

type ProjectViewInviteData struct {
	MarketingLayoutData

	SharedProjectData SharedProjectData
	ProjectId         primitive.ObjectID
	Token             projectInvite.Token
	Valid             bool
}

func (d *ProjectViewInviteData) Render() ([]byte, error) {
	return render("project/viewInvite.gohtml", 6*1024, d)
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

func (d *ProjectListData) Render() ([]byte, error) {
	n := 1024 * (38 + len(d.Projects)*3/4 + len(d.Tags)/10)
	return render("project/list.gohtml", n, d)
}

type EditorSettings struct {
	MaxDocLength           int64                  `json:"max_doc_length"`
	MaxEntitiesPerProject  int64                  `json:"maxEntitiesPerProject"`
	MaxUploadSize          int64                  `json:"maxUploadSize"`
	WikiEnabled            bool                   `json:"wikiEnabled"`
	WsURL                  string                 `json:"wsUrl"`
	WsRetryHandshake       int64                  `json:"wsRetryHandshake"`
	EnablePdfCaching       bool                   `json:"enablePdfCaching"`
	ResetServiceWorker     bool                   `json:"resetServiceWorker"`
	EditorThemes           []string               `json:"editorThemes"`
	TextExtensions         []sharedTypes.FileType `json:"textExtensions"`
	ValidRootDocExtensions []sharedTypes.FileType `json:"validRootDocExtensions"`
}

type EditorBootstrap struct {
	AllowedImageNames    []AllowedImageName           `json:"allowedImageNames"`
	Anonymous            bool                         `json:"anonymous"`
	AnonymousAccessToken project.AccessToken          `json:"anonymousAccessToken"`
	IsRestrictedUser     project.IsRestrictedUser     `json:"isRestrictedTokenMember"`
	JWTProject           string                       `json:"jwtCompile"`
	JWTLoggedInUser      string                       `json:"jwtLoggedInUser"`
	JWTSpelling          string                       `json:"jwtSpelling"`
	Project              project.LoadEditorViewPublic `json:"project"`
	RootDocPath          clsiTypes.RootResourcePath   `json:"rootDocPath"`
	User                 user.WithLoadEditorInfo      `json:"user"`
	WSBootstrap          WSBootstrap                  `json:"wsBootstrap"`
}

type WSBootstrap struct {
	JWT       string `json:"bootstrap"`
	ExpiresIn int64  `json:"expiresIn"`
}

type AllowedImageName struct {
	AdminOnly bool                  `json:"adminOnly"`
	Name      sharedTypes.ImageName `json:"name"`
	Desc      string                `json:"desc"`
}

type ProjectEditorData struct {
	AngularLayoutData
	EditorBootstrap *EditorBootstrap
}

func (d *ProjectEditorData) Entrypoint() string {
	return "frontend/js/ide.js"
}

func (d *ProjectEditorData) Meta() []metaEntry {
	out := d.AngularLayoutData.Meta()
	out = append(out, metaEntry{
		Name:    "ol-project_id",
		Content: d.EditorBootstrap.Project.Id.Hex(),
		Type:    stringContentType,
	})
	out = append(out, metaEntry{
		Name:    "ol-bootstrapEditor",
		Content: d.EditorBootstrap,
		Type:    jsonContentType,
	})
	out = append(out, metaEntry{
		Name:    "ol-publicEditorSettings",
		Content: d.Settings.EditorSettings,
		Type:    jsonContentType,
	})
	return out
}

func (d *ProjectEditorData) Render() ([]byte, error) {
	d.HideFooter = true
	d.HideNavBar = true
	return render("project/editor.gohtml", 67*1024, d)
}

func (d *ProjectEditorData) CSP() string {
	return d.Settings.CSPs.Editor
}

func (d *ProjectEditorData) ResourceHints() string {
	if d.ThemeModifier == "light-" {
		return resourceHints.ResourceHintsEditorLight()
	}
	return resourceHints.ResourceHintsEditorDefault()
}
