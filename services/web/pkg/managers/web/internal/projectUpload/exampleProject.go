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

package projectUpload

import (
	"context"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/projectUpload/exampleProjects"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) CreateExampleProject(ctx context.Context, request *types.CreateExampleProjectRequest, response *types.CreateExampleProjectResponse) error {
	if err := request.Session.CheckIsLoggedIn(); err != nil {
		return err
	}
	if err := request.Name.Validate(); err != nil {
		return err
	}

	exampleProject, errUnknownTemplate := exampleProjects.Get(request.Template)
	if errUnknownTemplate != nil {
		return errUnknownTemplate
	}
	viewData := &exampleProjects.ViewData{
		ProjectCreationDate: time.Now(),
		ProjectName:         request.Name,
		Owner:               request.Session.User.ToPublicUserInfo(),
	}

	files, err := exampleProject.Render(viewData)
	if err != nil {
		return errors.Tag(err, "cannot render template")
	}
	return m.CreateProject(ctx, &types.CreateProjectRequest{
		Files:              files,
		Name:               request.Name,
		SpellCheckLanguage: "inherit",
		UserId:             request.Session.User.Id,
	}, response)
}
