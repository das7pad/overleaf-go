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

package project

import (
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type JoinProjectViewPrivate struct {
	ForAuthorizationDetails
	ForTree
	JoinProjectViewPublic
	OwnerIdField
	RootDocField
}

type JoinProjectViewPublic struct {
	CompilerField
	DeletedDocsField
	IdField
	ImageNameField
	NameField
	OwnerFeaturesField
	SpellCheckLanguageField
	VersionField
}

type JoinProjectDetails struct {
	Project JoinProjectViewPrivate
	Owner   user.WithPublicInfo
}

type ListViewPrivate struct {
	ForAuthorizationDetails
	IdField
	LastUpdatedAtField
	LastUpdatedByField
	NameField
	OwnerIdField

	Owner       user.WithPublicInfo
	LastUpdater user.WithPublicInfo
}

type List []ListViewPrivate

type LoadEditorViewPrivate struct {
	LoadEditorViewPublic
	ForAuthorizationDetails
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

	treeIds        []sharedTypes.UUID
	treeKinds      []string
	treePaths      []string
	docSnapshots   []string
	createdAts     [][]byte
	hashes         []string
	sizes          []int64
	linkedFileData []LinkedFileData
}

type ForZip struct {
	NameField
	ForTree
}

type ForCreation struct {
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
	OwnerFeaturesField
}
