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

package appliedOps

import (
	"context"

	"github.com/go-redis/redis/v8"

	"github.com/das7pad/real-time/pkg/managers/realTime/internal/broadcaster"
	"github.com/das7pad/real-time/pkg/managers/realTime/internal/channel"
	"github.com/das7pad/real-time/pkg/types"
)

type Manager interface {
	broadcaster.Broadcaster
	ApplyUpdate(rpc *types.RPC) error
	AddComment(rpc *types.RPC) error
}

func New(ctx context.Context, client redis.UniversalClient) (Manager, error) {
	c, err := channel.New(ctx, client, "appliedOps")
	if err != nil {
		return nil, err
	}
	b := broadcaster.New(
		ctx,
		types.GetNextAppliedOpsClient,
		types.SetNextAppliedOpsClient,
		c,
	)
	return &manager{
		Broadcaster: b,
	}, nil
}

type manager struct {
	broadcaster.Broadcaster
}

func (m *manager) ApplyUpdate(rpc *types.RPC) error {
	panic("implement me")
}

func (m *manager) AddComment(rpc *types.RPC) error {
	panic("implement me")
}
