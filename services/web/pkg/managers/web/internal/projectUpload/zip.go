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
	"archive/zip"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/das7pad/overleaf-go/pkg/constants"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type zipFile struct {
	*zip.File
}

func (z *zipFile) Open() (io.ReadCloser, bool, error) {
	f, err := z.File.Open()
	return f, false, err
}

func (z *zipFile) Size() int64 {
	return int64(z.UncompressedSize64)
}

func (z *zipFile) Path() sharedTypes.PathName {
	return sharedTypes.PathName(z.Name)
}

func (z *zipFile) PreComputedHash() sharedTypes.Hash {
	return ""
}

func (z *zipFile) SourceElement() project.TreeElement {
	return nil
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
		return errors.Tag(errNewReader, "open zip")
	}
	if len(r.File) > 10_000 {
		return &errors.ValidationError{
			Msg: "too many entries in zip file (>10000)",
		}
	}

	files := make([]types.CreateProjectFile, 0, constants.MaxFilesPerProject)
	topDir := ""
	topDirSet := false
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
		if len(files) >= constants.MaxFilesPerProject {
			return &errors.ValidationError{
				Msg: "too many files for new project",
			}
		}
		files = append(files, &zipFile{File: file})
		if !topDirSet {
			if idx := strings.IndexByte(file.Name, '/'); idx != -1 {
				topDir = file.Name[:idx+1]
			}
			topDirSet = true
		}
		if topDir != "" && !strings.HasPrefix(file.Name, topDir) {
			topDir = ""
		}
	}
	if prefix := len(topDir); prefix != 0 {
		for _, file := range files {
			f := file.(*zipFile)
			f.Name = f.Name[prefix:]
		}
	}

	return m.CreateProject(ctx, &types.CreateProjectRequest{
		AddHeader:          request.AddHeader,
		Compiler:           request.Compiler,
		Files:              files,
		HasDefaultName:     request.HasDefaultName,
		Name:               request.Name,
		SpellCheckLanguage: "inherit",
		UserId:             request.Session.User.Id,
	}, response)
}
