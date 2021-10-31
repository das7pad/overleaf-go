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

package user

import (
	"fmt"

	"github.com/das7pad/overleaf-go/pkg/models/internal/views"
)

var (
	epochFieldProjection                = views.GetProjectionFor(EpochField{})
	featuresFieldProjection             = views.GetProjectionFor(FeaturesField{})
	projectListViewCaller               = views.GetProjectionFor(ProjectListViewCaller{})
	withLoadEditorInfoProjection        = views.GetProjectionFor(WithLoadEditorInfo{})
	withLoginInfoProjection             = views.GetProjectionFor(WithLoginInfo{})
	withPublicInfoAndFeaturesProjection = views.GetProjectionFor(WithPublicInfoAndFeatures{})
	withPublicInfo                      = views.GetProjectionFor(WithPublicInfo{})
)

func getProjection(model interface{}) views.View {
	switch model.(type) {
	case *EpochField:
		return epochFieldProjection
	case *FeaturesField:
		return featuresFieldProjection
	case *ProjectListViewCaller:
		return projectListViewCaller
	case WithPublicInfoAndFeatures:
		return withPublicInfoAndFeaturesProjection
	case *WithPublicInfoAndFeatures:
		return withPublicInfoAndFeaturesProjection
	case WithLoadEditorInfo:
		return withLoadEditorInfoProjection
	case *WithLoadEditorInfo:
		return withLoadEditorInfoProjection
	case []WithPublicInfo:
		return withPublicInfo
	case *WithLoginInfo:
		return withLoginInfoProjection
	}
	panic(fmt.Sprintf("missing projection for %v", model))
}
