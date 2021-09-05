// Golang port of the Overleaf docstore service
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

package models

type DocArchiveContents struct {
	DocLinesField  `bson:"inline"`
	DocRangesField `bson:"inline"`
}

type DocArchiveContext struct {
	DocArchiveContents `bson:"inline"`
	DocRevisionField   `bson:"inline"`

	// Fetch DocInS3Field as well.
	DocInS3Field `bson:"inline"`
}

type DocContents struct {
	DocIdField       `bson:"inline"`
	DocLinesField    `bson:"inline"`
	DocRevisionField `bson:"inline"`

	// Fetch DocInS3Field as well.
	DocInS3Field `bson:"inline"`
}

type DocContentsCollection []DocContents

type DocContentsWithFullContext struct {
	DocIdField       `bson:"inline"`
	DocDeletedField  `bson:"inline"`
	DocLinesField    `bson:"inline"`
	DocRangesField   `bson:"inline"`
	DocRevisionField `bson:"inline"`

	// Fetch DocInS3Field as well.
	DocInS3Field `bson:"inline"`

	// NOTE: DocOpsVersionField is not part of the `docs` collection entry.
	DocOpsVersionField
}

type DocLines struct {
	DocIdField    `bson:"inline"`
	DocLinesField `bson:"inline"`

	// Fetch DocInS3Field as well.
	DocInS3Field `bson:"inline"`
}

type DocMeta struct {
	DocNameField      `bson:"inline"`
	DocDeletedField   `bson:"inline"`
	DocDeletedAtField `bson:"inline"`
}

type DocName struct {
	DocIdField   `bson:"inline"`
	DocNameField `bson:"inline"`
}

type DocNameCollection []DocName

type DocRanges struct {
	DocIdField     `bson:"inline"`
	DocRangesField `bson:"inline"`

	// Fetch DocInS3Field as well.
	DocInS3Field `bson:"inline"`
}

type DocRangesCollection []DocRanges
