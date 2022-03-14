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

package project

import (
	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/models/user"
)

type JoinProjectViewPrivate struct {
	JoinProjectViewPublic   `edgedb:"inline"`
	ForAuthorizationDetails `edgedb:"inline"`
}
type JoinProjectViewPublic struct {
	IdField                 `edgedb:"inline"`
	CompilerField           `edgedb:"inline"`
	ImageNameField          `edgedb:"inline"`
	MembersField            `edgedb:"inline"`
	NameField               `edgedb:"inline"`
	RootDocIdField          `edgedb:"inline"`
	SpellCheckLanguageField `edgedb:"inline"`
	TrackChangesStateField  `edgedb:"inline"`
	TreeField               `edgedb:"inline"`
	VersionField            `edgedb:"inline"`
}

type ListViewPrivate struct {
	ArchivedField           `edgedb:"inline"`
	ForAuthorizationDetails `edgedb:"inline"`
	IdField                 `edgedb:"inline"`
	LastUpdatedAtField      `edgedb:"inline"`
	LastUpdatedByField      `edgedb:"inline"`
	NameField               `edgedb:"inline"`
	TrashedField            `edgedb:"inline"`
}

type LoadEditorViewPrivate struct {
	edgedb.Optional
	LoadEditorViewPublic    `edgedb:"inline"`
	ActiveField             `edgedb:"inline"`
	ForAuthorizationDetails `edgedb:"inline"`
	ForFolderPath           `edgedb:"inline"`
	RootDocForJumpField     `edgedb:"inline"`
}

type LoadEditorDetails struct {
	ProjectExists bool                    `edgedb:"project_exists"`
	Project       LoadEditorViewPrivate   `edgedb:"project"`
	User          user.WithLoadEditorInfo `edgedb:"user"`
}

type LoadEditorViewPublic struct {
	CompilerField  `edgedb:"inline"`
	IdField        `edgedb:"inline"`
	ImageNameField `edgedb:"inline"`
	NameField      `edgedb:"inline"`
	RootDocIdField `edgedb:"inline"`
	VersionField   `edgedb:"inline"`
}

type WithTokenMembers struct {
	AccessTokenReadAndWriteField `edgedb:"inline"`
	AccessTokenReadOnlyField     `edgedb:"inline"`
}

type WithInvitedMembers struct {
	AccessReadAndWriteField `edgedb:"inline"`
	AccessReadOnlyField     `edgedb:"inline"`
}

type WithMembers struct {
	WithTokenMembers   `edgedb:"inline"`
	WithInvitedMembers `edgedb:"inline"`
}

type ForAuthorizationDetails struct {
	WithMembers            `edgedb:"inline"`
	EpochField             `edgedb:"inline"`
	OwnerField             `edgedb:"inline"`
	PublicAccessLevelField `edgedb:"inline"`
	TokensField            `edgedb:"inline"`
}

type forTokenAccessCheck struct {
	IdField                 `edgedb:"inline"`
	ForAuthorizationDetails `edgedb:"inline"`
}

type withIdAndEpoch struct {
	IdField    `edgedb:"inline"`
	EpochField `edgedb:"inline"`
}

type withIdAndVersion struct {
	IdField      `edgedb:"inline"`
	VersionField `edgedb:"inline"`
}

type withIdAndEpochAndVersion struct {
	IdField      `edgedb:"inline"`
	EpochField   `edgedb:"inline"`
	VersionField `edgedb:"inline"`
}

type ForProjectInvite struct {
	IdField                 `edgedb:"inline"`
	NameField               `edgedb:"inline"`
	ForAuthorizationDetails `edgedb:"inline"`
}

type ForProjectOwnershipTransfer struct {
	IdField                 `edgedb:"inline"`
	NameField               `edgedb:"inline"`
	ForAuthorizationDetails `edgedb:"inline"`
}

type WithTree struct {
	TreeField    `edgedb:"inline"`
	VersionField `edgedb:"inline"`
}

type ForTree struct {
	VersionField    `edgedb:"inline"`
	RootFolderField `edgedb:"inline"`
	FoldersField    `edgedb:"inline"`
}

type ForFolderPath struct {
	RootFolderField     `edgedb:"inline"`
	FoldersForJumpField `edgedb:"inline"`
}

type WithTreeAndRootDoc struct {
	RootDocIdField `edgedb:"inline"`
	WithTree       `edgedb:"inline"`
}

type WithTreeAndAuth struct {
	ForAuthorizationDetails `edgedb:"inline"`
	WithTree                `edgedb:"inline"`
	NameField               `edgedb:"inline"`
}

type WithLastUpdatedDetails struct {
	LastUpdatedAtField `edgedb:"inline"`
	LastUpdatedByField `edgedb:"inline"`
}

type forMemberRemoval struct {
	WithMembers     `edgedb:"inline"`
	ArchivedByField `edgedb:"inline"`
	TrashedByField  `edgedb:"inline"`
}

type ForCreation struct {
	ActiveField             `edgedb:"inline"`
	CompilerField           `edgedb:"inline"`
	EpochField              `edgedb:"inline"`
	IdField                 `edgedb:"inline"`
	ImageNameField          `edgedb:"inline"`
	NameField               `edgedb:"inline"`
	LastUpdatedAtField      `edgedb:"inline"`
	LastUpdatedByField      `edgedb:"inline"`
	OwnerField              `edgedb:"inline"`
	PublicAccessLevelField  `edgedb:"inline"`
	RootDocField            `edgedb:"inline"`
	SpellCheckLanguageField `edgedb:"inline"`
	RootFolderField         `edgedb:"inline"`
	VersionField            `edgedb:"inline"`
}

type ForClone struct {
	ForAuthorizationDetails `edgedb:"inline"`
	CompilerField           `edgedb:"inline"`
	ImageNameField          `edgedb:"inline"`
	NameField               `edgedb:"inline"`
	RootDocIdField          `edgedb:"inline"`
	SpellCheckLanguageField `edgedb:"inline"`
	TreeField               `edgedb:"inline"`
	VersionField            `edgedb:"inline"`
}

type ForDeletion struct {
	IdField                 `edgedb:"inline"`
	ForAuthorizationDetails `edgedb:"inline"`
	ActiveField             `edgedb:"inline"`
	ArchivedByField         `edgedb:"inline"`
	AuditLogField           `edgedb:"inline"`
	CompilerField           `edgedb:"inline"`
	ImageNameField          `edgedb:"inline"`
	LastOpenedField         `edgedb:"inline"`
	LastUpdatedAtField      `edgedb:"inline"`
	LastUpdatedByField      `edgedb:"inline"`
	NameField               `edgedb:"inline"`
	RootDocIdField          `edgedb:"inline"`
	SpellCheckLanguageField `edgedb:"inline"`
	TrackChangesStateField  `edgedb:"inline"`
	TrashedByField          `edgedb:"inline"`
	TreeField               `edgedb:"inline"`
	VersionField            `edgedb:"inline"`
}
