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

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type IdField struct {
	Id primitive.ObjectID `json:"_id" bson:"_id"`
}

type ProjectIdField struct {
	ProjectId primitive.ObjectID `json:"project_id" bson:"project_id"`
}

type LinesField struct {
	Lines sharedTypes.Lines `json:"lines" bson:"lines"`
}

type RevisionField struct {
	Revision sharedTypes.Revision `json:"rev" bson:"rev"`
}

type NameField struct {
	Name sharedTypes.Filename `json:"name" bson:"name"`
}

type DeletedField struct {
	Deleted bool `json:"deleted" bson:"deleted"`
}

type DeletedAtField struct {
	DeletedAt time.Time `json:"deletedAt" bson:"deletedAt"`
}
type InS3Field struct {
	InS3 bool `json:"inS3" bson:"inS3"`
}

func (f InS3Field) IsArchived() bool {
	return f.InS3
}

type RangesField struct {
	Ranges sharedTypes.Ranges `json:"ranges" bson:"ranges"`
}

type VersionField struct {
	Version sharedTypes.Version `json:"version" bson:"version"`
}
