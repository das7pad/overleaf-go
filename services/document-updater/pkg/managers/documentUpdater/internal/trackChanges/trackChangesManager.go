// Golang port of Overleaf
// Copyright (C) 2021-2022 Jakob Ackermann <das7pad@outlook.com>
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

	"github.com/das7pad/overleaf-go/services/track-changes/pkg/managers/trackChanges/flush"
)

type Manager interface {
	FlushDocInBackground(projectId, docId edgedb.UUID)
	RecordAndFlushHistoryOps(ctx context.Context, projectId, docId edgedb.UUID, nUpdates, queueDepth int64) error
}

func New(c *edgedb.Client, client redis.UniversalClient) (Manager, error) {
	return flush.New(c, client)
}
