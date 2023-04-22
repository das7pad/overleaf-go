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

package compile

import (
	"context"

	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) WordCount(ctx context.Context, request *types.WordCountRequest, response *types.WordCountResponse) error {
	err := m.preprocessGenericPOST(
		request.SignedCompileProjectRequestOptions,
		request.ImageName,
		request,
	)
	if err != nil {
		return err
	}
	if m.bundle != nil {
		return m.bundle.WordCount(
			ctx, request.ProjectId, request.UserId,
			&request.WordCountRequest, response,
		)
	}
	return m.genericPOST(
		ctx,
		"/wordcount",
		request.SignedCompileProjectRequestOptions,
		request.ClsiServerId,
		&request.WordCountRequest,
		response,
	)
}
