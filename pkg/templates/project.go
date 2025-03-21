// Golang port of Overleaf
// Copyright (C) 2021-2024 Jakob Ackermann <das7pad@outlook.com>
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

	"github.com/das7pad/overleaf-go/pkg/errors"
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

	PostURL     *sharedTypes.URL
	ProjectName project.Name
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
	return render("project/tokenAccess.gohtml", 8*1024, d)
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
	Id                sharedTypes.UUID                    `json:"id"`
	Name              project.Name                        `json:"name"`
	LastUpdatedAt     time.Time                           `json:"lastUpdated"`
	LastUpdatedBy     user.WithPublicInfoAndNonStandardId `json:"lastUpdatedBy"`
	PublicAccessLevel project.PublicAccessLevel           `json:"publicAccessLevel"`
	AccessLevel       sharedTypes.PrivilegeLevel          `json:"accessLevel"`
	AccessSource      project.AccessSource                `json:"source"`
	Archived          bool                                `json:"archived"`
	Trashed           bool                                `json:"trashed"`
	Owner             user.WithPublicInfoAndNonStandardId `json:"owner"`
}

type PrefetchedProjectsBlob struct {
	TotalSize int                      `json:"totalSize"`
	Projects  []ProjectListProjectView `json:"projects"`
}

type ProjectListData struct {
	AngularLayoutData

	PrefetchedProjectsBlob PrefetchedProjectsBlob
	Tags                   []tag.Full
	JWTLoggedInUser        string
	Notifications          notification.Notifications
	UserEmails             []user.EmailDetailsWithDefaultFlag
	SuggestedLngCode       string
	SystemMessages         []systemMessage.Full
}

func (d *ProjectListData) Entrypoint() string {
	return "frontend/js/pages/project/list.js"
}

func (d *ProjectListData) Meta() []metaEntry {
	out := d.AngularLayoutData.Meta()
	out = append(out, metaEntry{
		Name:    "ol-prefetchedProjectsBlob",
		Content: d.PrefetchedProjectsBlob,
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
	out = append(out, metaEntry{
		Name:    "ol-emailConfirmationDisabled",
		Content: d.Settings.EmailConfirmationDisabled,
		Type:    jsonContentType,
	})
	return out
}

func (d *ProjectListData) Render() ([]byte, string, error) {
	n := 1024 * (10 + d.PrefetchedProjectsBlob.TotalSize*3/4 + len(d.Tags)/10)
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
	DetachRole           DetachRole                   `json:"detachRole"`
}

type DetachRole string

func (r DetachRole) Validate() error {
	switch r {
	case "detacher", "detached", "":
		return nil
	default:
		return &errors.ValidationError{Msg: "invalid detach role"}
	}
}

type AllowedImageName struct {
	AdminOnly bool                  `json:"adminOnly"`
	Name      sharedTypes.ImageName `json:"name"`
	Desc      sharedTypes.ImageYear `json:"desc"`
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
	d.DeferCSSBundleLoading = true
	d.HideFooter = true
	d.HideNavBar = true
	return render("project/editor.gohtml", 38*1024, d)
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

type ProjectEditorDetachedData struct {
	MarketingLayoutData
	EditorBootstrap *EditorBootstrap
}

func (d *ProjectEditorDetachedData) Entrypoint() string {
	return "frontend/js/pages/project/pdf-preview-detached.js"
}

func (d *ProjectEditorDetachedData) Meta() []metaEntry {
	return (&ProjectEditorData{
		AngularLayoutData: AngularLayoutData{
			CommonData: d.CommonData,
		},
		EditorBootstrap: d.EditorBootstrap,
	}).Meta()
}

func (d *ProjectEditorDetachedData) Render() ([]byte, string, error) {
	d.DeferCSSBundleLoading = true
	d.HideFooter = true
	d.HideNavBar = true
	return render("project/editorDetached.gohtml", 9*1024, d)
}

func (d *ProjectEditorDetachedData) CSP() string {
	return d.Settings.CSPs.Editor
}

func (d *ProjectEditorDetachedData) ResourceHints() string {
	if d.ThemeModifier == "light-" {
		return resourceHints.ResourceHintsEditorLight()
	}
	return resourceHints.ResourceHintsEditorDefault()
}
