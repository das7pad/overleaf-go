// Golang port of Overleaf
// Copyright (C) 2021-2024 Jakob Ackermann <das7pad@outlook.com>
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

package documentUpdater

import (
	"context"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/redisScanner"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater/internal/dispatchManager"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater/internal/docManager"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater/internal/realTimeRedisManager"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater/internal/trackChanges"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
	"github.com/das7pad/overleaf-go/services/track-changes/pkg/managers/trackChanges/flush"
)

type Manager interface {
	dispatchManager.Manager
	PeriodicFlushAll(ctx context.Context)
	PeriodicFlushAllHistory(ctx context.Context)
	CheckDocExists(ctx context.Context, projectId sharedTypes.UUID, docId sharedTypes.UUID) error
	GetDoc(ctx context.Context, projectId sharedTypes.UUID, docId sharedTypes.UUID, fromVersion sharedTypes.Version) (*types.GetDocResponse, error)
	GetProjectDocsAndFlushIfOldSnapshot(ctx context.Context, projectId sharedTypes.UUID) (types.DocContentSnapshots, error)
	FlushAll(ctx context.Context) (bool, error)
	FlushAndDeleteDoc(ctx context.Context, projectId, docId sharedTypes.UUID) error
	FlushProject(ctx context.Context, projectId sharedTypes.UUID) error
	FlushProjectInBackground(ctx context.Context, projectId sharedTypes.UUID) bool
	FlushAndDeleteProject(ctx context.Context, projectId sharedTypes.UUID) error
	SetDoc(ctx context.Context, projectId, docId sharedTypes.UUID, request types.SetDocRequest) error
	ProcessProjectUpdates(ctx context.Context, projectId sharedTypes.UUID, updates types.RenameDocUpdates) error
}

func New(options *types.Options, db *pgxpool.Pool, client redis.UniversalClient) (Manager, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	rtRm, err := realTimeRedisManager.New(client)
	if err != nil {
		return nil, err
	}
	tc, err := flush.NewPeriodic(db, client, options.PeriodicFlushAll)
	if err != nil {
		return nil, err
	}
	dm, err := docManager.New(db, client, tc, rtRm)
	if err != nil {
		return nil, err
	}
	return &manager{
		rc:                       client,
		dispatcher:               dispatchManager.New(options, client, dm, rtRm),
		dm:                       dm,
		tc:                       tc,
		rateLimitBackgroundFlush: make(chan struct{}, 50),
		pc:                       options.PeriodicFlushAll,
	}, nil
}

type dispatcher dispatchManager.Manager

type manager struct {
	dispatcher
	rc                       redis.UniversalClient
	dm                       docManager.Manager
	rateLimitBackgroundFlush chan struct{}
	pc                       redisScanner.PeriodicOptions
	tc                       trackChanges.Manager
}

func (m *manager) PeriodicFlushAll(ctx context.Context) {
	redisScanner.Periodic(
		ctx, m.rc, "DocsIn:{", m.pc,
		"document background flush", m.FlushProjectInBackground,
	)
}

func (m *manager) PeriodicFlushAllHistory(ctx context.Context) {
	m.tc.PeriodicFlushAll(ctx)
}

func (m *manager) FlushAll(ctx context.Context) (bool, error) {
	ok, err := redisScanner.Each(
		ctx, m.rc, "DocsIn:{", m.pc.Count,
		m.FlushProjectInBackground,
	)
	ok2, err2 := m.tc.FlushAll(ctx)
	return ok && ok2, errors.Merge(err, err2)
}

func (m *manager) ProcessProjectUpdates(ctx context.Context, projectId sharedTypes.UUID, updates types.RenameDocUpdates) error {
	if err := updates.Validate(); err != nil {
		return err
	}

	for _, update := range updates {
		err := m.dm.RenameDoc(ctx, projectId, update.DocId, update.NewPath)
		if err != nil {
			return errors.Tag(err, update.DocId.String())
		}
	}
	return nil
}

func (m *manager) CheckDocExists(ctx context.Context, projectId, docId sharedTypes.UUID) error {
	_, err := m.dm.GetDoc(ctx, projectId, docId)
	return err
}

func (m *manager) GetDoc(ctx context.Context, projectId, docId sharedTypes.UUID, fromVersion sharedTypes.Version) (*types.GetDocResponse, error) {
	response := types.GetDocResponse{}
	if fromVersion == -1 {
		doc, err := m.dm.GetDoc(ctx, projectId, docId)
		if err != nil {
			return nil, err
		}
		response.Ops = make([]sharedTypes.DocumentUpdate, 0)
		response.PathName = doc.PathName
		response.Snapshot = string(doc.Snapshot)
		response.Version = doc.Version
	} else {
		doc, updates, err := m.dm.GetDocAndRecentUpdates(
			ctx, projectId, docId, fromVersion,
		)
		if err != nil {
			return nil, err
		}
		response.Ops = updates
		response.Version = doc.Version
	}
	return &response, nil
}

func (m *manager) GetProjectDocsAndFlushIfOldSnapshot(ctx context.Context, projectId sharedTypes.UUID) (types.DocContentSnapshots, error) {
	docs, err := m.dm.GetProjectDocsAndFlushIfOld(ctx, projectId)
	if err != nil {
		return nil, err
	}
	docContentsSnapshot := make(types.DocContentSnapshots, len(docs))
	for i, doc := range docs {
		docContentsSnapshot[i] = doc.ToDocContentSnapshot()
	}
	return docContentsSnapshot, nil
}

func (m *manager) SetDoc(ctx context.Context, projectId, docId sharedTypes.UUID, request types.SetDocRequest) error {
	return m.dm.SetDoc(ctx, projectId, docId, request)
}

func (m *manager) FlushAndDeleteDoc(ctx context.Context, projectId, docId sharedTypes.UUID) error {
	return m.dm.FlushAndDeleteDoc(ctx, projectId, docId)
}

func (m *manager) FlushProject(ctx context.Context, projectId sharedTypes.UUID) error {
	return m.dm.FlushProject(ctx, projectId)
}

func (m *manager) FlushProjectInBackground(ctx context.Context, projectId sharedTypes.UUID) bool {
	m.rateLimitBackgroundFlush <- struct{}{}
	defer func() { <-m.rateLimitBackgroundFlush }()

	ctx, done := context.WithTimeout(ctx, 30*time.Second)
	defer done()
	if err := m.FlushAndDeleteProject(ctx, projectId); err != nil {
		log.Printf("background flush failed: %s: %s", projectId, err)
		return false
	}
	return true
}

func (m *manager) FlushAndDeleteProject(ctx context.Context, projectId sharedTypes.UUID) error {
	return m.dm.FlushAndDeleteProject(ctx, projectId)
}
