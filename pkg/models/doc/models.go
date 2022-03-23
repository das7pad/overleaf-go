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

import (
	"time"

	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type forInsertion struct {
	IdField        `edgedb:"$inline"`
	LinesField     `edgedb:"$inline"`
	ProjectIdField `edgedb:"$inline"`
	RangesField    `edgedb:"$inline"`
	VersionField   `edgedb:"$inline"`
	RevisionField  `edgedb:"$inline"`
}

type ForDocUpdate struct {
	Snapshot sharedTypes.Snapshot
	VersionField
	RangesField
	LastUpdatedAt time.Time   `json:"lastUpdatedAt"`
	LastUpdatedBy edgedb.UUID `json:"lastUpdatedBy"`
}

type ArchiveContents struct {
	LinesField  `edgedb:"$inline"`
	RangesField `edgedb:"$inline"`
}

type ArchiveContext struct {
	ArchiveContents `edgedb:"$inline"`
	RevisionField   `edgedb:"$inline"`

	// Fetch InS3Field as well.
	InS3Field `edgedb:"$inline"`
}

type Contents struct {
	IdField       `edgedb:"$inline"`
	LinesField    `edgedb:"$inline"`
	RevisionField `edgedb:"$inline"`

	// Fetch InS3Field as well.
	InS3Field `edgedb:"$inline"`
}

type ContentsCollection []*Contents

type ContentsWithFullContext struct {
	IdField       `edgedb:"$inline"`
	DeletedField  `edgedb:"$inline"`
	LinesField    `edgedb:"$inline"`
	RangesField   `edgedb:"$inline"`
	RevisionField `edgedb:"$inline"`

	// Fetch InS3Field as well.
	InS3Field `edgedb:"$inline"`

	VersionField `edgedb:"$inline"`
}

type Lines struct {
	IdField    `edgedb:"$inline"`
	LinesField `edgedb:"$inline"`

	// Fetch InS3Field as well.
	InS3Field `edgedb:"$inline"`
}

type Meta struct {
	NameField      `edgedb:"$inline"`
	DeletedField   `edgedb:"$inline"`
	DeletedAtField `edgedb:"$inline"`
}

type Name struct {
	IdField   `edgedb:"$inline"`
	NameField `edgedb:"$inline"`
}

type NameCollection []Name

type Ranges struct {
	IdField     `edgedb:"$inline"`
	RangesField `edgedb:"$inline"`

	// Fetch InS3Field as well, but hide it from the frontend.
	InS3Field `json:"-" edgedb:"$inline"`
}

type RangesCollection []Ranges
