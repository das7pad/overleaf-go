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

package editor

import (
	"context"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) GetProjectFile(ctx context.Context, request *types.GetProjectFileRequest, response *types.GetProjectFileResponse) error {
	projectId := request.ProjectId
	fileId := request.FileId
	userId := request.Session.User.Id
	token := request.Session.GetAnonTokenAccess(projectId)
	f, err := m.pm.GetFile(ctx, projectId, userId, token, fileId)
	if err != nil {
		return errors.Tag(err, "cannot get file tree")
	}
	s, r, err := m.fm.GetReadStreamForProjectFile(ctx, projectId, fileId)
	if err != nil {
		return errors.Tag(err, "cannot get filestream")
	}
	response.Filename = f.Name
	response.Reader = r
	response.Size = s
	return nil
}

func (m *manager) GetProjectFileSize(ctx context.Context, request *types.GetProjectFileSizeRequest, response *types.GetProjectFileSizeResponse) error {
	projectId := request.ProjectId
	fileId := request.FileId
	userId := request.Session.User.Id
	token := request.Session.GetAnonTokenAccess(projectId)
	f, err := m.pm.GetFile(ctx, projectId, userId, token, fileId)
	if err != nil {
		return errors.Tag(err, "cannot get file tree")
	}
	response.Filename = f.Name
	response.Size = f.Size
	return nil
}
