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
	"archive/zip"
	"context"
	"fmt"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type zipFile struct {
	*zip.File
}

func (z *zipFile) Size() int64 {
	return int64(z.UncompressedSize64)
}

func (z *zipFile) Path() sharedTypes.PathName {
	return sharedTypes.PathName(z.Name)
}

func (m *manager) CreateFromZip(ctx context.Context, request *types.CreateProjectFromZipRequest, response *types.CreateProjectResponse) error {
	request.Preprocess()
	if err := request.Validate(); err != nil {
		return err
	}
	if err := request.Session.CheckIsLoggedIn(); err != nil {
		return err
	}

	r, errNewReader := zip.NewReader(request.File, request.Size)
	if errNewReader != nil {
		return errors.Tag(errNewReader, "cannot open zip")
	}
	if len(r.File) > 10*1000 {
		return &errors.ValidationError{
			Msg: "too many entries in zip file (>10000)",
		}
	}

	files := make([]types.CreateProjectFile, 0, len(r.File))
	for _, file := range r.File {
		mode := file.Mode()
		if mode.IsDir() {
			continue
		}
		if !mode.IsRegular() {
			return &errors.ValidationError{
				Msg: fmt.Sprintf("%q is not a dir/file", file.Name),
			}
		}
		files = append(files, &zipFile{File: file})
	}

	return m.CreateProject(ctx, &types.CreateProjectRequest{
		AddHeader:      request.AddHeader,
		Files:          files,
		HasDefaultName: request.HasDefaultName,
		Name:           request.Name,
		UserId:         request.Session.User.Id,
	}, response)
}
