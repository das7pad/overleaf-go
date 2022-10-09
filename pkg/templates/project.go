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

package templates

import (
	"time"

	"github.com/das7pad/overleaf-go/pkg/models/notification"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/projectInvite"
	"github.com/das7pad/overleaf-go/pkg/models/systemMessage"
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

func (d *ProjectTokenAccessData) Render() ([]byte, string, error) {
	return render("project/tokenAccess.gohtml", 6*1024, d)
}

type ProjectViewInviteData struct {
	MarketingLayoutData

	SharedProjectData SharedProjectData
	ProjectId         sharedTypes.UUID
	Token             projectInvite.Token
	Valid             bool
}

func (d *ProjectViewInviteData) Render() ([]byte, string, error) {
	return render("project/viewInvite.gohtml", 6*1024, d)
}

type ProjectListProjectView struct {
	Id                  sharedTypes.UUID           `json:"id"`
	Name                project.Name               `json:"name"`
	LastUpdatedAt       *time.Time                 `json:"lastUpdated"`
	LastUpdatedByUserId sharedTypes.UUID           `json:"-"`
	LastUpdatedBy       user.WithPublicInfo        `json:"lastUpdatedBy"`
	PublicAccessLevel   project.PublicAccessLevel  `json:"publicAccessLevel"`
	AccessLevel         sharedTypes.PrivilegeLevel `json:"accessLevel"`
	AccessSource        project.AccessSource       `json:"source"`
	Archived            bool                       `json:"archived"`
	Trashed             bool                       `json:"trashed"`
	OwnerRef            sharedTypes.UUID           `json:"owner_ref"`
	Owner               user.WithPublicInfo        `json:"owner"`
}

type ProjectListData struct {
	AngularLayoutData

	Projects         []*ProjectListProjectView
	Tags             []tag.Full
	JWTLoggedInUser  string
	Notifications    notification.Notifications
	UserEmails       []user.EmailDetailsWithDefaultFlag
	SuggestedLngCode string
	SystemMessages   []systemMessage.Full
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
	out = append(out, metaEntry{
		Name:    "ol-notifications",
		Content: d.Notifications,
		Type:    jsonContentType,
	})
	out = append(out, metaEntry{
		Name:    "ol-systemMessages",
		Content: d.SystemMessages,
		Type:    jsonContentType,
	})
	return out
}

func (d *ProjectListData) Render() ([]byte, string, error) {
	n := 1024 * (38 + len(d.Projects)*3/4 + len(d.Tags)/10)
	return render("project/list.gohtml", n, d)
}

type EditorSettings struct {
	MaxDocLength           int64                  `json:"max_doc_length"`
	MaxEntitiesPerProject  int64                  `json:"maxEntitiesPerProject"`
	MaxUploadSize          int64                  `json:"maxUploadSize"`
	WsURL                  string                 `json:"wsUrl"`
	WsRetryHandshake       int64                  `json:"wsRetryHandshake"`
	EditorThemes           []string               `json:"editorThemes"`
	TextExtensions         []sharedTypes.FileType `json:"textExtensions"`
	ValidRootDocExtensions []sharedTypes.FileType `json:"validRootDocExtensions"`
	WikiEnabled            bool                   `json:"wikiEnabled"`
	EnablePdfCaching       bool                   `json:"enablePdfCaching"`
	ResetServiceWorker     bool                   `json:"resetServiceWorker"`
}

type EditorBootstrap struct {
	AllowedImageNames    []AllowedImageName           `json:"allowedImageNames"`
	AnonymousAccessToken project.AccessToken          `json:"anonymousAccessToken"`
	SystemMessages       []systemMessage.Full         `json:"systemMessages"`
	JWTProject           string                       `json:"jwtCompile"`
	JWTLoggedInUser      string                       `json:"jwtLoggedInUser"`
	JWTSpelling          string                       `json:"jwtSpelling"`
	Project              project.LoadEditorViewPublic `json:"project"`
	RootDocPath          sharedTypes.PathName         `json:"rootDocPath"`
	User                 user.WithLoadEditorInfo      `json:"user"`
	Anonymous            bool                         `json:"anonymous"`
	IsRestrictedUser     project.IsRestrictedUser     `json:"isRestrictedTokenMember"`
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
		Content: d.EditorBootstrap.Project.Id.String(),
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
	out = append(out, metaEntry{
		Name:    "ol-siteURL",
		Content: d.Settings.SiteURL.String(),
		Type:    stringContentType,
	})
	return out
}

func (d *ProjectEditorData) Render() ([]byte, string, error) {
	d.HideFooter = true
	d.HideNavBar = true
	return render("project/editor.gohtml", 43*1024, d)
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
