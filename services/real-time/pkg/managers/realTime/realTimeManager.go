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

package realTime

import (
	"context"

	"github.com/das7pad/real-time/pkg/errors"
	"github.com/das7pad/real-time/pkg/types"
)

type Manager interface {
	PeriodicCleanup(ctx context.Context)

	RPC(ctx context.Context, client *types.Client, request *types.RPCRequest, response *types.RPCResponse) error
}

func New(options *types.Options) (Manager, error) {
	return &manager{options: options}, nil
}

type manager struct {
	options *types.Options
}

func (m *manager) PeriodicCleanup(ctx context.Context) {
	<-ctx.Done()
}

func (m *manager) RPC(ctx context.Context, client *types.Client, request *types.RPCRequest, response *types.RPCResponse) error {
	err := m.rpc(ctx, client, request, response)
	if err == nil {
		return nil
	}
	if errors.IsValidationError(err) {
		response.Error = err.Error()
		return nil
	}
	if errors.IsInvalidState(err) {
		response.Error = err.Error()
		return err
	}
	response.Error = "Something went wrong in real-time service"
	return err
}

func (m *manager) rpc(ctx context.Context, client *types.Client, request *types.RPCRequest, response *types.RPCResponse) error {
	if err := client.CanDo(request.Action, request.DocId); err != nil {
		return err
	}
	response.Body = request.Body
	return nil
}
