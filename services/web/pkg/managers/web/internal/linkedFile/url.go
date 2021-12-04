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

package linkedFile

import (
	"context"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) fromURL(ctx context.Context, request *types.CreateLinkedFileRequest) error {
	f, err := m.proxy.DownloadFile(ctx, request.Parameter.URL)
	if err != nil {
		return errors.Tag(err, "cannot download file")
	}
	defer f.Cleanup()

	uploadDetails := f.ToUploadDetails()
	uploadDetails.FileName = request.Name
	return m.ftm.UploadFile(ctx, &types.UploadFileRequest{
		ProjectId:      request.ProjectId,
		UserId:         request.UserId,
		ParentFolderId: request.ParentFolderId,
		LinkedFileData: request.LinkedFileData(),
		UploadDetails:  uploadDetails,
	})
}

func (m *manager) refreshURL(ctx context.Context, request *types.RefreshLinkedFileRequest) error {
	return m.fromURL(ctx, &types.CreateLinkedFileRequest{
		UserId:         request.UserId,
		ProjectId:      request.ProjectId,
		ParentFolderId: request.ParentFolderId,
		Name:           request.File.Name,
		Provider:       request.File.LinkedFileData.Provider,
		Parameter: types.CreateLinkedFileProviderParameter{
			URL: request.File.LinkedFileData.URL,
		},
	})
}
