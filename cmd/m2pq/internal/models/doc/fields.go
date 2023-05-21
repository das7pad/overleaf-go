// Golang port of Overleaf
// Copyright (C) 2021-2023 Jakob Ackermann <das7pad@outlook.com>
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
	Id primitive.ObjectID `bson:"_id"`
}

type ProjectIdField struct {
	ProjectId primitive.ObjectID `bson:"project_id"`
}

type LinesField struct {
	Lines []string `bson:"lines"`
}

type NameField struct {
	Name sharedTypes.Filename `bson:"name"`
}

type DeletedField struct {
	Deleted bool `bson:"deleted"`
}

type DeletedAtField struct {
	DeletedAt time.Time `bson:"deletedAt"`
}

type InS3Field struct {
	InS3 bool `bson:"inS3"`
}

type VersionField struct {
	Version sharedTypes.Version `bson:"version"`
}
