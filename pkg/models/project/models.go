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
	JoinProjectViewPublic `bson:"inline"`
	Members               `bson:"inline"`
	OwnerRefField         `bson:"inline"`
}
type JoinProjectViewPublic struct {
	IdField                 `bson:"inline"`
	CompilerField           `bson:"inline"`
	ImageNameField          `bson:"inline"`
	NameField               `bson:"inline"`
	PublicAccessLevelField  `bson:"inline"`
	RootDocIdField          `bson:"inline"`
	SpellCheckLanguageField `bson:"inline"`
	TokensField             `bson:"inline"`
	TrackChangesStateField  `bson:"inline"`
	TreeField               `bson:"inline"`
}

type Members struct {
	CollaboratorRefsField            `bson:"inline"`
	ReadOnlyRefsField                `bson:"inline"`
	TokenAccessReadAndWriteRefsField `bson:"inline"`
	TokenAccessReadOnlyRefsField     `bson:"inline"`
}

type WithTree struct {
	TreeField `bson:"inline"`
}

type WithLastUpdatedDetails struct {
	LastUpdatedAtField `bson:"inline"`
	LastUpdatedByField `bson:"inline"`
}
