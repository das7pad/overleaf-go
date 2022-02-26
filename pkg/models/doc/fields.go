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

type IdField struct {
	Id edgedb.UUID `json:"_id" edgedb:"id"`
}

type ProjectIdField struct {
	ProjectId edgedb.UUID `json:"project_id" edgedb:"project_id"`
}

type LinesField struct {
	Lines sharedTypes.Lines `json:"lines" edgedb:"lines"`
}

type RevisionField struct {
	Revision sharedTypes.Revision `json:"rev" edgedb:"rev"`
}

type NameField struct {
	Name sharedTypes.Filename `json:"name" edgedb:"name"`
}

type DeletedField struct {
	Deleted bool `json:"deleted" edgedb:"deleted"`
}

type DeletedAtField struct {
	DeletedAt time.Time `json:"deletedAt" edgedb:"deletedAt"`
}
type InS3Field struct {
	InS3 bool `json:"inS3" edgedb:"inS3"`
}

func (f InS3Field) IsArchived() bool {
	return f.InS3
}

type RangesField struct {
	Ranges sharedTypes.Ranges `json:"ranges" edgedb:"ranges"`
}

type VersionField struct {
	Version sharedTypes.Version `json:"version" edgedb:"version"`
}
