// Golang port of Overleaf
// Copyright (C) 2022-2024 Jakob Ackermann <das7pad@outlook.com>
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
	"log"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/das7pad/overleaf-go/pkg/models/docHistory"
	"github.com/das7pad/overleaf-go/pkg/redisLocker"
	"github.com/das7pad/overleaf-go/pkg/redisScanner"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type PeriodicManager interface {
	Manager
	PeriodicFlushAll(ctx context.Context)
	FlushAll(ctx context.Context) (bool, error)
}

type Manager interface {
	FlushDoc(ctx context.Context, projectId, docId sharedTypes.UUID) error
	FlushDocInBackground(projectId, docId sharedTypes.UUID)
	FlushProject(ctx context.Context, projectId sharedTypes.UUID) error
	RecordAndFlushHistoryOps(ctx context.Context, projectId, docId sharedTypes.UUID, nUpdates, queueDepth int64) error
}

func NewPeriodic(db *pgxpool.Pool, client redis.UniversalClient, pc redisScanner.PeriodicOptions) (PeriodicManager, error) {
	m, err := newManager(db, client)
	if err != nil {
		return nil, err
	}
	return &periodicManager{
		manager: m,
		pc:      pc,
	}, nil
}

func New(db *pgxpool.Pool, client redis.UniversalClient) (Manager, error) {
	return newManager(db, client)
}

func newManager(db *pgxpool.Pool, client redis.UniversalClient) (*manager, error) {
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

type periodicManager struct {
	*manager
	pc redisScanner.PeriodicOptions
}

func (m *periodicManager) PeriodicFlushAll(ctx context.Context) {
	redisScanner.Periodic(
		ctx, m.client, "DocsWithHistoryOps:{", m.pc,
		"history background flush", m.FlushProjectInBackground,
	)
}

func (m *periodicManager) FlushAll(ctx context.Context) (bool, error) {
	return redisScanner.Each(
		ctx, m.client, "DocsWithHistoryOps:{",
		m.pc.Count, m.FlushProjectInBackground,
	)
}

func (m *manager) FlushProjectInBackground(ctx context.Context, projectId sharedTypes.UUID) bool {
	ctx, done := context.WithTimeout(ctx, 30*time.Second)
	defer done()
	if err := m.FlushProject(ctx, projectId); err != nil {
		log.Printf("history background flush failed: %s: %s", projectId, err)
		return false
	}
	return true
}
