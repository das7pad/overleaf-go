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

package project

import (
	"fmt"

	"github.com/das7pad/overleaf-go/pkg/models/internal/views"
)

var (
	epochFieldProjection              = views.GetProjectionFor(EpochField{})
	lastUpdatedAtFieldProjection      = views.GetProjectionFor(LastUpdatedAtField{})
	forAuthorizationDetailsProjection = views.GetProjectionFor(ForAuthorizationDetails{})
	withTreeProjection                = views.GetProjectionFor(WithTree{})
	withMembersProjection             = views.GetProjectionFor(WithMembers{})
	withTokenMembersProjection        = views.GetProjectionFor(WithTokenMembers{})
	joinProjectViewPrivateProjection  = views.GetProjectionFor(JoinProjectViewPrivate{})
	listViewPrivateProjection         = views.GetProjectionFor(ListViewPrivate{})
	loadEditorViewPrivateProjection   = views.GetProjectionFor(LoadEditorViewPrivate{})
	withTreeAndAuth                   = views.GetProjectionFor(WithTreeAndAuth{})
)

func getProjection(model interface{}) views.View {
	switch model.(type) {
	case *EpochField:
		return epochFieldProjection
	case *ForAuthorizationDetails:
		return forAuthorizationDetailsProjection
	case *LastUpdatedAtField:
		return lastUpdatedAtFieldProjection
	case JoinProjectViewPrivate:
		return joinProjectViewPrivateProjection
	case *JoinProjectViewPrivate:
		return joinProjectViewPrivateProjection
	case LoadEditorViewPrivate:
		return loadEditorViewPrivateProjection
	case *LoadEditorViewPrivate:
		return loadEditorViewPrivateProjection
	case []ListViewPrivate:
		return listViewPrivateProjection
	case WithMembers:
		return withMembersProjection
	case WithTokenMembers:
		return withTokenMembersProjection
	case WithTree:
		return withTreeProjection
	case *WithTreeAndAuth:
		return withTreeAndAuth
	}
	panic(fmt.Sprintf("missing projection for %v", model))
}
