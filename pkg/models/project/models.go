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

type JoinProjectViewPrivate struct {
	JoinProjectViewPublic   `bson:"inline"`
	ForAuthorizationDetails `bson:"inline"`
}
type JoinProjectViewPublic struct {
	IdField                 `bson:"inline"`
	CompilerField           `bson:"inline"`
	ImageNameField          `bson:"inline"`
	NameField               `bson:"inline"`
	RootDocIdField          `bson:"inline"`
	SpellCheckLanguageField `bson:"inline"`
	TrackChangesStateField  `bson:"inline"`
	TreeField               `bson:"inline"`
	VersionField            `bson:"inline"`
}

type ListViewPrivate struct {
	ArchivedByField         `bson:"inline"`
	ForAuthorizationDetails `bson:"inline"`
	IdField                 `bson:"inline"`
	LastUpdatedAtField      `bson:"inline"`
	LastUpdatedByField      `bson:"inline"`
	NameField               `bson:"inline"`
	TrashedByField          `bson:"inline"`
}

type LoadEditorViewPrivate struct {
	LoadEditorViewPublic    `bson:"inline"`
	ActiveField             `bson:"inline"`
	ForAuthorizationDetails `bson:"inline"`
	TreeField               `bson:"inline"`
}

type LoadEditorViewPublic struct {
	CompilerField  `bson:"inline"`
	IdField        `bson:"inline"`
	ImageNameField `bson:"inline"`
	NameField      `bson:"inline"`
	RootDocIdField `bson:"inline"`
	VersionField   `bson:"inline"`
}

type WithTokenMembers struct {
	TokenAccessReadAndWriteRefsField `bson:"inline"`
	TokenAccessReadOnlyRefsField     `bson:"inline"`
}

type WithInvitedMembers struct {
	CollaboratorRefsField `bson:"inline"`
	ReadOnlyRefsField     `bson:"inline"`
}

type WithMembers struct {
	WithTokenMembers   `bson:"inline"`
	WithInvitedMembers `bson:"inline"`
}

type ForAuthorizationDetails struct {
	WithMembers            `bson:"inline"`
	EpochField             `bson:"inline"`
	OwnerRefField          `bson:"inline"`
	PublicAccessLevelField `bson:"inline"`
	TokensField            `bson:"inline"`
}

type forTokenAccessCheck struct {
	IdField                 `bson:"inline"`
	ForAuthorizationDetails `bson:"inline"`
}

type withIdAndEpoch struct {
	IdField    `bson:"inline"`
	EpochField `bson:"inline"`
}

type withIdAndVersion struct {
	IdField      `bson:"inline"`
	VersionField `bson:"inline"`
}

type withIdAndEpochAndVersion struct {
	IdField      `bson:"inline"`
	EpochField   `bson:"inline"`
	VersionField `bson:"inline"`
}

type ForProjectInvite struct {
	IdField                 `bson:"inline"`
	NameField               `bson:"inline"`
	ForAuthorizationDetails `bson:"inline"`
}

type ForProjectOwnershipTransfer struct {
	IdField                 `bson:"inline"`
	NameField               `bson:"inline"`
	ForAuthorizationDetails `bson:"inline"`
}

type WithTree struct {
	TreeField    `bson:"inline"`
	VersionField `bson:"inline"`
}

type WithTreeAndRootDoc struct {
	RootDocIdField `bson:"inline"`
	WithTree       `bson:"inline"`
}

type WithTreeAndAuth struct {
	ForAuthorizationDetails `bson:"inline"`
	WithTree                `bson:"inline"`
	NameField               `bson:"inline"`
}

type WithLastUpdatedDetails struct {
	LastUpdatedAtField `bson:"inline"`
	LastUpdatedByField `bson:"inline"`
}

type forMemberRemoval struct {
	WithMembers     `bson:"inline"`
	ArchivedByField `bson:"inline"`
	TrashedByField  `bson:"inline"`
}

type ForCreation struct {
	ActiveField             `bson:"inline"`
	CompilerField           `bson:"inline"`
	EpochField              `bson:"inline"`
	IdField                 `bson:"inline"`
	ImageNameField          `bson:"inline"`
	NameField               `bson:"inline"`
	LastUpdatedAtField      `bson:"inline"`
	LastUpdatedByField      `bson:"inline"`
	OwnerRefField           `bson:"inline"`
	PublicAccessLevelField  `bson:"inline"`
	RootDocIdField          `bson:"inline"`
	SpellCheckLanguageField `bson:"inline"`
	TreeField               `bson:"inline"`
	VersionField            `bson:"inline"`
}

type ForClone struct {
	ForAuthorizationDetails `bson:"inline"`
	CompilerField           `bson:"inline"`
	ImageNameField          `bson:"inline"`
	NameField               `bson:"inline"`
	RootDocIdField          `bson:"inline"`
	SpellCheckLanguageField `bson:"inline"`
	TreeField               `bson:"inline"`
	VersionField            `bson:"inline"`
}

type ForDeletion struct {
	IdField                 `bson:"inline"`
	ForAuthorizationDetails `bson:"inline"`
	ActiveField             `bson:"inline"`
	ArchivedByField         `bson:"inline"`
	CompilerField           `bson:"inline"`
	ImageNameField          `bson:"inline"`
	LastOpenedField         `bson:"inline"`
	LastUpdatedAtField      `bson:"inline"`
	LastUpdatedByField      `bson:"inline"`
	NameField               `bson:"inline"`
	RootDocIdField          `bson:"inline"`
	SpellCheckLanguageField `bson:"inline"`
	TrackChangesStateField  `bson:"inline"`
	TrashedByField          `bson:"inline"`
	TreeField               `bson:"inline"`
	VersionField            `bson:"inline"`
}
