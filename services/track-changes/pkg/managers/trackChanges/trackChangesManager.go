// Golang port of Overleaf
// Copyright (C) 2022 Jakob Ackermann <das7pad@outlook.com>
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

package trackChanges

import (
	"context"

	"github.com/edgedb/edgedb-go"
	"github.com/go-redis/redis/v8"

	"github.com/das7pad/overleaf-go/pkg/models/docHistory"
	"github.com/das7pad/overleaf-go/services/track-changes/pkg/managers/trackChanges/flush"
	"github.com/das7pad/overleaf-go/services/track-changes/pkg/managers/trackChanges/updates"
	"github.com/das7pad/overleaf-go/services/track-changes/pkg/types"
)

type Manager interface {
	flushManager
	updatesManager
	GetDocDiff(ctx context.Context, request *types.GetDocDiffRequest, response *types.GetDocDiffResponse) error
	RestoreDocVersion(ctx context.Context, request *types.RestoreDocVersionRequest) error
}

func New(c *edgedb.Client, client redis.UniversalClient) (Manager, error) {
	fm, err := flush.New(c, client)
	if err != nil {
		return nil, err
	}
	um := updates.New(docHistory.New(c), fm)
	return &manager{
		flushManager:   fm,
		updatesManager: um,
	}, nil
}

type flushManager = flush.Manager
type updatesManager = updates.Manager

type manager struct {
	flushManager
	updatesManager
}

func (m *manager) GetDocDiff(ctx context.Context, request *types.GetDocDiffRequest, response *types.GetDocDiffResponse) error {
	// TODO implement me
	panic("implement me")
}

func (m *manager) RestoreDocVersion(ctx context.Context, request *types.RestoreDocVersionRequest) error {
	// TODO implement me
	panic("implement me")
}
