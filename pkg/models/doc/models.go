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

package doc

type forInsertion struct {
	IdField        `bson:"inline"`
	LinesField     `bson:"inline"`
	ProjectIdField `bson:"inline"`
	RangesField    `bson:"inline"`
	VersionField   `bson:"inline"`
	RevisionField  `bson:"inline"`
}

type ForDocUpdate struct {
	LinesField   `bson:"inline"`
	RangesField  `bson:"inline"`
	VersionField `bson:"inline"`
}

type ArchiveContents struct {
	LinesField  `bson:"inline"`
	RangesField `bson:"inline"`
}

type ArchiveContext struct {
	ArchiveContents `bson:"inline"`
	RevisionField   `bson:"inline"`

	// Fetch InS3Field as well.
	InS3Field `bson:"inline"`
}

type Contents struct {
	IdField       `bson:"inline"`
	LinesField    `bson:"inline"`
	RevisionField `bson:"inline"`

	// Fetch InS3Field as well.
	InS3Field `bson:"inline"`
}

type ContentsCollection []*Contents

type ContentsWithFullContext struct {
	IdField       `bson:"inline"`
	DeletedField  `bson:"inline"`
	LinesField    `bson:"inline"`
	RangesField   `bson:"inline"`
	RevisionField `bson:"inline"`

	// Fetch InS3Field as well.
	InS3Field `bson:"inline"`

	VersionField `bson:"inline"`
}

type Lines struct {
	IdField    `bson:"inline"`
	LinesField `bson:"inline"`

	// Fetch InS3Field as well.
	InS3Field `bson:"inline"`
}

type Meta struct {
	NameField      `bson:"inline"`
	DeletedField   `bson:"inline"`
	DeletedAtField `bson:"inline"`
}

type Name struct {
	IdField   `bson:"inline"`
	NameField `bson:"inline"`
}

type NameCollection []Name

type Ranges struct {
	IdField     `bson:"inline"`
	RangesField `bson:"inline"`

	// Fetch InS3Field as well, but hide it from the frontend.
	InS3Field `json:"-" bson:"inline"`
}

type RangesCollection []Ranges
