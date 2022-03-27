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

package projectDownload

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) CreateMultiProjectZIP(ctx context.Context, request *types.CreateMultiProjectZIPRequest, response *types.CreateProjectZIPResponse) error {
	if err := request.Validate(); err != nil {
		return err
	}

	buffer, err := os.CreateTemp("", "zip-download")
	if err != nil {
		return errors.Tag(err, "cannot create buffer")
	}
	response.FSPath = buffer.Name()
	f := zip.NewWriter(buffer)

	for _, projectId := range request.ProjectIds {
		err = m.createProjectZIP(ctx, &types.CreateProjectZIPRequest{
			Session:   request.Session,
			ProjectId: projectId,
		}, func(filename sharedTypes.Filename) (io.Writer, error) {
			return f.Create(string(filename))
		})
		if err != nil {
			err = errors.Tag(err, "project: "+projectId.String())
			break
		}
	}
	errCloseZIP := f.Close()
	errCloseBuffer := buffer.Close()
	if err != nil {
		return err
	}
	if errCloseZIP != nil {
		return errors.Tag(errCloseZIP, "cannot close zip")
	}
	if errCloseBuffer != nil {
		return errors.Tag(errCloseBuffer, "cannot close buffer")
	}
	response.Filename = sharedTypes.Filename(fmt.Sprintf(
		"Overleaf Projects (%d items).zip", len(request.ProjectIds),
	))
	return nil
}
