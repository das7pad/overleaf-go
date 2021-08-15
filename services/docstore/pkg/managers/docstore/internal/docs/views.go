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
	"fmt"

	"github.com/das7pad/docstore/pkg/managers/docstore/internal/docs/internal/views"
	"github.com/das7pad/docstore/pkg/models"
)

var (
	docArchiveContentsFields = views.GetFieldsOf(models.DocArchiveContents{})

	docArchiveContextProjection          = views.GetProjectionFor(models.DocArchiveContext{})
	docContentsProjection                = views.GetProjectionFor(models.DocContents{})
	docContentsWithFullContextProjection = views.GetProjectionFor(models.DocContentsWithFullContext{})
	docDeletedFieldProjection            = views.GetProjectionFor(models.DocDeletedField{})
	docIdFieldProjection                 = views.GetProjectionFor(models.DocIdField{})
	docInS3FieldProjection               = views.GetProjectionFor(models.DocInS3Field{})
	docNameProjection                    = views.GetProjectionFor(models.DocName{})
	docRangesProjection                  = views.GetProjectionFor(models.DocRanges{})
	docRevisionFieldProjection           = views.GetProjectionFor(models.DocRevisionField{})
	docOpsVersionFieldProjection         = views.GetProjectionFor(models.DocOpsVersionField{})
)

func getProjection(model interface{}) views.View {
	switch model.(type) {
	case models.DocArchiveContext:
		return docArchiveContextProjection
	case models.DocContentsCollection:
		return docContentsProjection
	case models.DocContentsWithFullContext:
		return docContentsWithFullContextProjection
	case models.DocDeletedField:
		return docDeletedFieldProjection
	case models.DocInS3Field:
		return docInS3FieldProjection
	case models.DocNameCollection:
		return docNameProjection
	case models.DocRangesCollection:
		return docRangesProjection
	case models.DocRevisionField:
		return docRevisionFieldProjection
	case models.DocOpsVersionField:
		return docOpsVersionFieldProjection
	}
	panic(fmt.Sprintf("missing projection for %v", model))
}
