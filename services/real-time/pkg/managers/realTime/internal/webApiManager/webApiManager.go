// Golang port of the Overleaf real-time service
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

package webApiManager

import (
	"context"

	"github.com/das7pad/real-time/pkg/types"
)

type Manager interface {
	JoinProject(ctx context.Context, client *types.Client, request *types.JoinProjectRequest) (*types.JoinProjectWebApiResponse, error)
}

func New(options *types.Options) Manager {
	return &manager{options: options}
}

type manager struct {
	options *types.Options
}

func (m *manager) JoinProject(ctx context.Context, client *types.Client, request *types.JoinProjectRequest) (*types.JoinProjectWebApiResponse, error) {
	panic("implement me")
}
