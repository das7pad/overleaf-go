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

package project

import (
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type ForBootstrapWS struct {
	ForTree `json:"-"`

	CompilerField
	ContentLockedAtField
	EditableField
	DeletedDocsField
	IdField
	ImageNameField
	NameField
	OwnerField
	PublicAccessLevelField
	RootDocIdField
	SpellCheckLanguageField
	VersionField
}

type ListViewPrivate struct {
	ContentLockedAtField
	ForAuthorizationDetails
	IdField
	LastUpdatedAtField
	LastUpdatedByField
	NameField
	OwnerIdField

	LastUpdater user.WithPublicInfoAndNonStandardId
	Owner       user.WithPublicInfoAndNonStandardId
	TagIds      sharedTypes.UUIDs
}

type List []ListViewPrivate

type WithIdAndName struct {
	IdField
	NameField
}

type LoadEditorViewPrivate struct {
	LoadEditorViewPublic
	ForAuthorizationDetails
	EditableField
	RootDocField
}

type LoadEditorDetails struct {
	Project LoadEditorViewPrivate
	User    user.WithLoadEditorInfo
}

type LoadEditorViewPublic struct {
	CompilerField
	IdField
	ImageNameField
	NameField
	OwnerFeaturesField
	RootDocIdField
	VersionField
}

type ForAuthorizationDetails struct {
	Member
	EpochField
	PublicAccessLevelField
	TokensField
}

type ForTokenAccessDetails struct {
	IdField
	NameField
	ForAuthorizationDetails
}

type ForProjectInvite struct {
	NameField
	ForAuthorizationDetails

	Sender user.WithPublicInfo
	User   user.WithPublicInfo
}

type ForProjectEntries struct {
	DocsField
	FilesField
}

type ForTree struct {
	RootFolderField

	treeIds        sharedTypes.UUIDs
	treeKinds      []TreeNodeKind
	treePaths      []string
	docSnapshots   []string
	createdAts     []pgtype.Timestamp
	hashes         []string
	sizes          []int64
	linkedFileData []*LinkedFileData
}

type ForZip struct {
	NameField
	ForTree
}

type ForCreation struct {
	CreatedAtField
	CompilerField
	DeletedAtField
	IdField
	ImageNameField
	NameField
	OwnerIdField
	RootDocField
	SpellCheckLanguageField
	RootFolderField
}

type ForClone struct {
	ForAuthorizationDetails
	CompilerField
	ImageNameField
	RootDocField
	SpellCheckLanguageField
	ForTree
}

type ForProjectJWT struct {
	ForAuthorizationDetails
	EditableField
	OwnerFeaturesField
}
