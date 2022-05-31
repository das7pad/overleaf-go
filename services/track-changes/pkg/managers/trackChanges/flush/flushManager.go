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

package flush

import (
	"context"
	"database/sql"

	"github.com/go-redis/redis/v8"

	"github.com/das7pad/overleaf-go/pkg/models/docHistory"
	"github.com/das7pad/overleaf-go/pkg/redisLocker"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Manager interface {
	FlushDoc(ctx context.Context, projectId, docId sharedTypes.UUID) error
	FlushDocInBackground(projectId, docId sharedTypes.UUID)
	FlushProject(ctx context.Context, projectId sharedTypes.UUID) error
	RecordAndFlushHistoryOps(ctx context.Context, projectId, docId sharedTypes.UUID, nUpdates, queueDepth int64) error
}

func New(db *sql.DB, client redis.UniversalClient) (Manager, error) {
	rl, err := redisLocker.New(client, "HistoryLock")
	if err != nil {
		return nil, err
	}
	return &manager{
		client: client,
		dhm:    docHistory.New(db),
		rl:     rl,
	}, nil
}

type manager struct {
	client redis.UniversalClient
	dhm    docHistory.Manager
	rl     redisLocker.Locker
}
