// Golang port of Overleaf
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

package doc

import (
	"fmt"

	"github.com/das7pad/overleaf-go/pkg/models/docOps"
	"github.com/das7pad/overleaf-go/pkg/models/internal/views"
)

var (
	docArchiveContentsFields = views.GetFieldsOf(ArchiveContents{})

	docArchiveContextProjection          = views.GetProjectionFor(ArchiveContext{})
	docContentsProjection                = views.GetProjectionFor(Contents{})
	docContentsWithFullContextProjection = views.GetProjectionFor(ContentsWithFullContext{})
	docDeletedFieldProjection            = views.GetProjectionFor(DeletedField{})
	docIdFieldProjection                 = views.GetProjectionFor(IdField{})
	docInS3FieldProjection               = views.GetProjectionFor(InS3Field{})
	docNameProjection                    = views.GetProjectionFor(Name{})
	docRangesProjection                  = views.GetProjectionFor(Ranges{})
	docRevisionFieldProjection           = views.GetProjectionFor(RevisionField{})
	docOpsVersionFieldProjection         = views.GetProjectionFor(docOps.VersionField{})
)

func getProjection(model interface{}) views.View {
	switch model.(type) {
	case ArchiveContext:
		return docArchiveContextProjection
	case ContentsCollection:
		return docContentsProjection
	case ContentsWithFullContext:
		return docContentsWithFullContextProjection
	case DeletedField:
		return docDeletedFieldProjection
	case InS3Field:
		return docInS3FieldProjection
	case NameCollection:
		return docNameProjection
	case RangesCollection:
		return docRangesProjection
	case RevisionField:
		return docRevisionFieldProjection
	case docOps.VersionField:
		return docOpsVersionFieldProjection
	}
	panic(fmt.Sprintf("missing projection for %v", model))
}
