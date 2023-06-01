// Golang port of Overleaf
// Copyright (C) 2023 Jakob Ackermann <das7pad@outlook.com>
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
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type publicAccessLevelChangedBody struct {
	NewAccessLevel project.PublicAccessLevel `json:"newAccessLevel"`
}

func (m *manager) SetPublicAccessLevel(ctx context.Context, request *types.SetPublicAccessLevelRequest, response *types.SetPublicAccessLevelResponse) error {
	if err := request.PublicAccessLevel.Validate(); err != nil {
		return err
	}

	if request.PublicAccessLevel == project.TokenBasedAccess {
		t, err := m.pm.PopulateTokens(ctx, request.ProjectId, request.UserId)
		if err != nil {
			return errors.Tag(err, "populate tokens")
		}
		response.Tokens = t
	}

	err := m.pm.SetPublicAccessLevel(
		ctx, request.ProjectId, request.UserId, request.PublicAccessLevel,
	)
	if err != nil {
		return errors.Tag(err, "update PublicAccessLevel")
	}

	go m.notifyEditor(
		request.ProjectId,
		"project:publicAccessLevel:changed",
		publicAccessLevelChangedBody{
			NewAccessLevel: request.PublicAccessLevel,
		},
	)
	return nil
}

func (m *manager) GetAccessTokens(ctx context.Context, request *types.GetAccessTokensRequest, response *types.GetAccessTokensResponse) error {
	t := project.Tokens{}
	err := m.pm.GetAccessTokens(ctx, request.ProjectId, request.UserId, &t)
	if err != nil {
		return err
	}
	response.Tokens = &t
	return nil
}
