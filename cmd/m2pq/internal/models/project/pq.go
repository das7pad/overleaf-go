// Golang port of Overleaf
// Copyright (C) 2022 Jakob Ackermann <das7pad@outlook.com>
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

type ForPQ struct {
	ActiveField                      `bson:"inline"`
	ArchivedByField                  `bson:"inline"`
	AuditLogField                    `bson:"inline"`
	CollaboratorRefsField            `bson:"inline"`
	CompilerField                    `bson:"inline"`
	EpochField                       `bson:"inline"`
	IdField                          `bson:"inline"`
	ImageNameField                   `bson:"inline"`
	LastOpenedField                  `bson:"inline"`
	LastUpdatedAtField               `bson:"inline"`
	LastUpdatedByField               `bson:"inline"`
	NameField                        `bson:"inline"`
	OwnerRefField                    `bson:"inline"`
	PublicAccessLevelField           `bson:"inline"`
	ReadOnlyRefsField                `bson:"inline"`
	RootDocIdField                   `bson:"inline"`
	SpellCheckLanguageField          `bson:"inline"`
	TokenAccessReadAndWriteRefsField `bson:"inline"`
	TokenAccessReadOnlyRefsField     `bson:"inline"`
	TokensField                      `bson:"inline"`
	TrashedByField                   `bson:"inline"`
	TreeField                        `bson:"inline"`
	VersionField                     `bson:"inline"`
}
