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
	"net/url"

	"github.com/das7pad/overleaf-go/pkg/errors"
	clsiTypes "github.com/das7pad/overleaf-go/services/clsi/pkg/types"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/compile"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) fromProjectOutputFile(ctx context.Context, r *types.CreateLinkedFileRequest) error {
	u := m.pdfDownloadBase.WithPath(
		string(clsiTypes.BuildDownloadPath(
			r.Parameter.SourceProjectId, r.UserId,
			r.Parameter.BuildId, r.Parameter.SourceOutputFilePath,
		)),
	).WithQuery(url.Values{
		compile.ClsiServerIdQueryParam: {string(r.Parameter.ClsiServerId)},
	})

	f, err := m.proxy.DownloadFile(ctx, u)
	if err != nil {
		if errors.IsUnprocessableEntityError(err) {
			return &errors.UnprocessableEntityError{
				Msg: "output file is not available for importing",
			}
		}
		return errors.Tag(err, "cannot download file")
	}
	defer f.Cleanup()

	uploadDetails := f.ToUploadDetails()
	uploadDetails.FileName = r.Name
	return m.ftm.UploadFile(ctx, &types.UploadFileRequest{
		ProjectId:      r.ProjectId,
		UserId:         r.UserId,
		ParentFolderId: r.ParentFolderId,
		LinkedFileData: r.LinkedFileData(),
		UploadDetails:  uploadDetails,
	})
}

func (m *manager) refreshProjectOutputFile(ctx context.Context, r *types.RefreshLinkedFileRequest) error {
	res := types.CompileProjectResponse{}
	err := m.cm.CompileHeadLess(ctx, &types.CompileProjectHeadlessRequest{
		ProjectId: r.File.LinkedFileData.SourceProjectId,
		UserId:    r.UserId,
	}, &res)
	if err != nil {
		return err
	}
	if res.Status != "success" {
		return &errors.UnprocessableEntityError{Msg: "compile request failed"}
	}

	path := r.File.LinkedFileData.SourceOutputFilePath
	var buildId clsiTypes.BuildId
	for _, f := range res.OutputFiles {
		if f.Path == path {
			buildId = f.Build
			break
		}
	}
	if buildId == "" {
		return &errors.UnprocessableEntityError{Msg: "file not found"}
	}

	return m.fromProjectOutputFile(ctx, &types.CreateLinkedFileRequest{
		WithProjectIdAndUserId: r.WithProjectIdAndUserId,
		ParentFolderId:         r.ParentFolderId,
		Name:                   r.File.Name,
		Provider:               r.File.LinkedFileData.Provider,
		Parameter: types.CreateLinkedFileProviderParameter{
			BuildId:              buildId,
			ClsiServerId:         res.ClsiServerId,
			SourceOutputFilePath: path,
			SourceProjectId:      r.File.LinkedFileData.SourceProjectId,
		},
	})
}
