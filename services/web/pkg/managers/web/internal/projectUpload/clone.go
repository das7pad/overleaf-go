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

package projectUpload

import (
	"context"
	"io"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type cloneProjectFile struct {
	project.TreeElement
}

func (c cloneProjectFile) Path() sharedTypes.PathName {
	switch el := c.TreeElement.(type) {
	case project.Doc:
		return el.Path
	case project.FileRef:
		return el.Path
	default:
		return ""
	}
}

func (c cloneProjectFile) Size() int64 {
	return 0
}

func (c cloneProjectFile) Open() (io.ReadCloser, bool, error) {
	return nil, false, errors.New("must clone instead")
}

func (c cloneProjectFile) PreComputedHash() sharedTypes.Hash {
	return ""
}

func (c cloneProjectFile) SourceElement() project.TreeElement {
	return c.TreeElement
}

func (m *manager) CloneProject(ctx context.Context, request *types.CloneProjectRequest, response *types.CloneProjectResponse) error {
	if err := request.Session.CheckIsLoggedIn(); err != nil {
		return err
	}
	if err := request.Name.Validate(); err != nil {
		return err
	}
	sourceProjectId := request.ProjectId
	userId := request.Session.User.Id

	if _, err := m.pm.GetAuthorizationDetails(ctx, sourceProjectId, userId, ""); err != nil {
		return errors.Tag(err, "check auth")
	}
	if err := m.dum.FlushProject(ctx, sourceProjectId); err != nil {
		return errors.Tag(err, "flush docs to db")
	}

	p, err := m.pm.GetForClone(ctx, sourceProjectId, userId)
	if err != nil {
		return errors.Tag(err, "get source project")
	}
	rootDocPath, elements, folders := p.BuildTreeElements()
	files := make([]types.CreateProjectFile, len(elements))
	for i, d := range elements {
		files[i] = cloneProjectFile{TreeElement: d}
	}
	return m.CreateProject(ctx, &types.CreateProjectRequest{
		Compiler:           p.Compiler,
		Files:              files,
		ExtraFolders:       folders,
		ImageName:          p.ImageName,
		Name:               request.Name,
		RootDocPath:        rootDocPath,
		SourceProjectId:    sourceProjectId,
		SpellCheckLanguage: p.SpellCheckLanguage,
		UserId:             request.Session.User.Id,
	}, response)
}
