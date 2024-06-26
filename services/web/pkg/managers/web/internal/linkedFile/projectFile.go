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

package linkedFile

import (
	"context"
	"io"
	"os"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) fromProjectFile(ctx context.Context, request *types.CreateLinkedFileRequest) error {
	sourceProjectId := request.Parameter.SourceProjectId
	userId := request.UserId
	elementId, isDoc, err := m.pm.GetElementByPath(
		ctx, sourceProjectId, userId, request.Parameter.SourceEntityPath,
	)
	if err != nil {
		return errors.Tag(err, "get source element")
	}

	f, err := os.CreateTemp("", "linked-file")
	if err != nil {
		return errors.Tag(err, "create temp file")
	}
	defer func() {
		_ = f.Close()
		_ = os.Remove(f.Name())
	}()

	var size int64
	if isDoc {
		d, err2 := m.dum.GetDoc(ctx, sourceProjectId, elementId, -1)
		if err2 != nil {
			return errors.Tag(err2, "get doc")
		}
		n, err2 := f.WriteString(d.Snapshot)
		if err2 != nil {
			return errors.Tag(err2, "buffer doc")
		}
		size = int64(n)
	} else {
		_, reader, err2 := m.fm.GetReadStreamForProjectFile(
			ctx, sourceProjectId, elementId,
		)
		if err2 != nil {
			return errors.Tag(err2, "get file")
		}
		size, err = io.Copy(f, reader)
		_ = reader.Close()
		if err != nil {
			return errors.Tag(err, "buffer file")
		}
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return errors.Tag(err, "reset buffer to start")
	}

	return m.ftm.UploadFile(ctx, &types.UploadFileRequest{
		ProjectId:      request.ProjectId,
		UserId:         request.UserId,
		ParentFolderId: request.ParentFolderId,
		LinkedFileData: request.LinkedFileData(),
		ClientId:       request.ClientId,
		UploadDetails: types.UploadDetails{
			File:     f,
			FileName: request.Name,
			Size:     size,
		},
	})
}

func (m *manager) refreshProjectFile(ctx context.Context, r *types.RefreshLinkedFileRequest) error {
	return m.fromProjectFile(ctx, &types.CreateLinkedFileRequest{
		WithProjectIdAndUserId: r.WithProjectIdAndUserId,
		ParentFolderId:         r.ParentFolderId,
		Name:                   r.File.Name,
		Provider:               r.File.LinkedFileData.Provider,
		Parameter: types.CreateLinkedFileProviderParameter{
			SourceProjectId:  r.File.LinkedFileData.SourceProjectId,
			SourceEntityPath: r.File.LinkedFileData.SourceEntityPath,
		},
		ClientId: r.ClientId,
	})
}
