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

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type DocIdField struct {
	Id primitive.ObjectID `json:"_id" bson:"_id"`
}

type DocLinesField struct {
	Lines Lines `json:"lines" bson:"lines"`
}

type DocRevisionField struct {
	Revision Revision `json:"rev" bson:"rev"`
}

type DocNameField struct {
	Name string `json:"name" bson:"name"`
}

type DocDeletedField struct {
	Deleted bool `json:"deleted" bson:"deleted"`
}

type DocDeletedAtField struct {
	DeletedAt time.Time `json:"deletedAt" bson:"deletedAt"`
}
type DocInS3Field struct {
	InS3 bool `json:"inS3" bson:"inS3"`
}

func (f DocInS3Field) IsArchived() bool {
	return f.InS3
}

type DocOpsVersionField struct {
	Version Version `json:"version" bson:"version"`
}

type DocOpsDocIdField struct {
	DocId primitive.ObjectID `bson:"doc_id"`
}

type DocRangesField struct {
	Ranges Ranges `json:"ranges" bson:"ranges"`
}
