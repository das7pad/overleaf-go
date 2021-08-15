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

package docs

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/docstore/pkg/models"
)

func docFilter(projectId primitive.ObjectID, docId primitive.ObjectID) bson.M {
	return bson.M{
		"project_id": projectId,
		"_id":        docId,
	}
}
func docFilterWithRevision(projectId primitive.ObjectID, docId primitive.ObjectID, revision models.Revision) bson.M {
	filter := docFilter(projectId, docId)
	filter["rev"] = revision
	return filter
}

func projectFilterAllDocs(projectId primitive.ObjectID) bson.M {
	return bson.M{
		"project_id": projectId,
	}
}

func projectFilterDeleted(projectId primitive.ObjectID) bson.M {
	filter := projectFilterAllDocs(projectId)
	filter["deleted"] = true
	return filter
}

func projectFilterNonArchivedDocs(projectId primitive.ObjectID) bson.M {
	filter := projectFilterAllDocs(projectId)
	filter["inS3"] = bson.M{
		"$ne": true,
	}
	return filter
}

func projectFilterNonDeleted(projectId primitive.ObjectID) bson.M {
	filter := projectFilterAllDocs(projectId)
	filter["deleted"] = bson.M{
		"$ne": true,
	}
	return filter
}

func projectFilterNeedsUnArchiving(projectId primitive.ObjectID) bson.M {
	filter := projectFilterNonDeleted(projectId)
	filter["inS3"] = true
	return filter
}
