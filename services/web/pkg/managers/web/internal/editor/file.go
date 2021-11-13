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

package editor

import (
	"context"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/objectStorage"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) GetProjectFile(ctx context.Context, request *types.GetProjectFileRequest, response *types.GetProjectFileResponse) error {
	projectId := request.ProjectId
	fileId := request.FileId
	userId := request.Session.User.Id
	token := request.Session.GetAnonTokenAccess(projectId)
	p, err := m.pm.GetTreeAndAuth(ctx, projectId, userId)
	if err != nil {
		return errors.Tag(err, "cannot get file tree")
	}
	if _, err = p.GetPrivilegeLevel(userId, token); err != nil {
		return err
	}
	t, err := p.GetRootFolder()
	if err != nil {
		return err
	}

	var name sharedTypes.Filename
	err = t.WalkFiles(func(e project.TreeElement, path sharedTypes.PathName) error {
		if e.GetId() == fileId {
			name = e.GetName()
			return project.AbortWalk
		}
		return nil
	})
	if name == "" {
		return &errors.NotFoundError{}
	}

	o := objectStorage.GetOptions{}
	s, r, err := m.fm.GetReadStreamForProjectFile(ctx, projectId, fileId, o)
	if err != nil {
		return errors.Tag(err, "cannot get filestream")
	}
	response.Filename = name
	response.Reader = r
	response.Size = s
	return nil
}

func (m *manager) GetProjectFileSize(ctx context.Context, request *types.GetProjectFileSizeRequest, response *types.GetProjectFileSizeResponse) error {
	projectId := request.ProjectId
	fileId := request.FileId
	userId := request.Session.User.Id
	token := request.Session.GetAnonTokenAccess(projectId)
	_, err := m.pm.GetAuthorizationDetails(ctx, projectId, userId, token)
	if err != nil {
		return err
	}

	s, err := m.fm.GetSizeOfProjectFile(ctx, projectId, fileId)
	if err != nil {
		return errors.Tag(err, "cannot get file size")
	}
	response.Size = s
	return nil
}
