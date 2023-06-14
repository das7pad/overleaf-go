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

package openInOverleaf

import (
	"context"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) createFromZip(ctx context.Context, request *types.OpenInOverleafRequest, response *types.CreateProjectResponse) error {
	f, err := m.proxy.DownloadFile(ctx, request.ZipURL)
	if err != nil {
		return errors.Tag(err, "download file")
	}
	defer f.Cleanup()

	if request.HasDefaultName {
		request.ProjectName = project.Name(f.Path().Filename().Basename())
	}
	return m.pum.CreateFromZip(ctx, &types.CreateProjectFromZipRequest{
		AddHeader:      m.addHeader,
		Compiler:       request.Compiler,
		WithSession:    types.WithSession{Session: request.Session},
		HasDefaultName: request.HasDefaultName,
		Name:           request.ProjectName,
		UploadDetails:  f.ToUploadDetails(),
	}, response)
}
