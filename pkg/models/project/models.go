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
)

type JoinProjectViewPrivate struct {
	ForAuthorizationDetails `edgedb:"$inline"`
	ForTree                 `edgedb:"$inline"`
	JoinProjectViewPublic   `edgedb:"$inline"`
	RootDocField            `edgedb:"$inline"`
}

type JoinProjectViewPublic struct {
	CompilerField           `edgedb:"$inline"`
	DeletedDocsField        `edgedb:"$inline"`
	IdField                 `edgedb:"$inline"`
	ImageNameField          `edgedb:"$inline"`
	InvitesField            `edgedb:"$inline"`
	NameField               `edgedb:"$inline"`
	SpellCheckLanguageField `edgedb:"$inline"`
}

type JoinProjectDetails struct {
	ProjectExists bool                   `edgedb:"project_exists"`
	Project       JoinProjectViewPrivate `edgedb:"project"`
}

type ListViewPrivate struct {
	ArchivedByField         `edgedb:"$inline"`
	ForAuthorizationDetails `edgedb:"$inline"`
	IdField                 `edgedb:"$inline"`
	LastUpdatedAtField      `edgedb:"$inline"`
	LastUpdatedByField      `edgedb:"$inline"`
	NameField               `edgedb:"$inline"`
	TrashedByField          `edgedb:"$inline"`
}

type LoadEditorViewPrivate struct {
	LoadEditorViewPublic    `edgedb:"$inline"`
	ForAuthorizationDetails `edgedb:"$inline"`
	RootDocField            `edgedb:"$inline"`
}

type LoadEditorDetails struct {
	ProjectExists bool                    `edgedb:"project_exists"`
	Project       LoadEditorViewPrivate   `edgedb:"project"`
	User          user.WithLoadEditorInfo `edgedb:"user"`
}

type LoadEditorViewPublic struct {
	CompilerField  `edgedb:"$inline"`
	IdField        `edgedb:"$inline"`
	ImageNameField `edgedb:"$inline"`
	NameField      `edgedb:"$inline"`
	RootDocIdField `edgedb:"$inline"`
	VersionField   `edgedb:"$inline"`
}

type WithTokenMembers struct {
	AccessTokenReadAndWriteField `edgedb:"$inline"`
	AccessTokenReadOnlyField     `edgedb:"$inline"`
}

type WithInvitedMembers struct {
	AccessReadAndWriteField `edgedb:"$inline"`
	AccessReadOnlyField     `edgedb:"$inline"`
}

type WithMembers struct {
	WithInvitedMembers `edgedb:"$inline"`
	WithTokenMembers   `edgedb:"$inline"`
}

type ForAuthorizationDetails struct {
	WithMembers            `edgedb:"$inline"`
	EpochField             `edgedb:"$inline"`
	OwnerField             `edgedb:"$inline"`
	PublicAccessLevelField `edgedb:"$inline"`
	TokensField            `edgedb:"$inline"`
}

type forTokenAccessCheck struct {
	IdField                 `edgedb:"$inline"`
	ForAuthorizationDetails `edgedb:"$inline"`
}

type ForProjectInvite struct {
	IdField                 `edgedb:"$inline"`
	NameField               `edgedb:"$inline"`
	ForAuthorizationDetails `edgedb:"$inline"`
}

type ForProjectEntries struct {
	DocsField  `edgedb:"$inline"`
	FilesField `edgedb:"$inline"`
}

type ForTree struct {
	VersionField       `edgedb:"$inline"`
	RootFolderField    `edgedb:"$inline"`
	FoldersField       `edgedb:"$inline"`
	rootFolderResolved bool
}

type ForZip struct {
	NameField `edgedb:"$inline"`
	ForTree   `edgedb:"$inline"`
}

type ForCreation struct {
	CompilerField           `edgedb:"$inline"`
	DeletedAtField          `edgedb:"$inline"`
	EpochField              `edgedb:"$inline"`
	IdField                 `edgedb:"$inline"`
	ImageNameField          `edgedb:"$inline"`
	NameField               `edgedb:"$inline"`
	LastUpdatedAtField      `edgedb:"$inline"`
	LastUpdatedByField      `edgedb:"$inline"`
	OwnerField              `edgedb:"$inline"`
	PublicAccessLevelField  `edgedb:"$inline"`
	RootDocField            `edgedb:"$inline"`
	SpellCheckLanguageField `edgedb:"$inline"`
	RootFolderField         `edgedb:"$inline"`
	VersionField            `edgedb:"$inline"`
}

type ForClone struct {
	ForAuthorizationDetails `edgedb:"$inline"`
	CompilerField           `edgedb:"$inline"`
	ImageNameField          `edgedb:"$inline"`
	RootDocField            `edgedb:"$inline"`
	SpellCheckLanguageField `edgedb:"$inline"`
	DocsField               `edgedb:"$inline"`
	FilesField              `edgedb:"$inline"`
}
