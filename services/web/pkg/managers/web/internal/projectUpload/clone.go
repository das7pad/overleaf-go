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
		return el.ResolvedPath
	case project.FileRef:
		return el.ResolvedPath
	default:
		return ""
	}
}

func (c cloneProjectFile) Size() int64 {
	return 0
}

func (c cloneProjectFile) Open() (io.ReadCloser, error) {
	return nil, errors.New("must clone instead")
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
		return errors.Tag(err, "cannot check auth")
	}
	if err := m.dum.FlushProject(ctx, sourceProjectId); err != nil {
		return errors.Tag(err, "cannot flush docs to mongo")
	}

	sp := &project.ForClone{}
	if err := m.pm.GetProject(ctx, sourceProjectId, sp); err != nil {
		return errors.Tag(err, "cannot get source project")
	}
	if _, err := sp.GetPrivilegeLevelAuthenticated(userId); err != nil {
		return err
	}

	nDocs := len(sp.Docs)
	files := make([]types.CreateProjectFile, nDocs+len(sp.Files))
	for i, d := range sp.Docs {
		files[i] = cloneProjectFile{TreeElement: d}
	}
	for i, file := range sp.Files {
		files[nDocs+i] = cloneProjectFile{TreeElement: file}
	}
	return m.CreateProject(ctx, &types.CreateProjectRequest{
		Compiler:           sp.Compiler,
		Files:              files,
		ImageName:          sp.ImageName,
		Name:               request.Name,
		RootDocPath:        sp.RootDoc.ResolvedPath,
		SourceProjectId:    sourceProjectId,
		SpellCheckLanguage: sp.SpellCheckLanguage,
		UserId:             request.Session.User.Id,
	}, response)
}
