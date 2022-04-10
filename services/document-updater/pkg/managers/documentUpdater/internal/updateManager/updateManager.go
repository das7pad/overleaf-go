// Golang port of Overleaf
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

package updateManager

import (
	"context"
	"time"

	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"

	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater/internal/realTimeRedisManager"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater/internal/redisManager"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/sharejs/types/text"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
)

type Manager interface {
	ProcessOutstandingUpdates(
		ctx context.Context,
		docId edgedb.UUID,
		doc *types.Doc,
		transformUpdatesCache []sharedTypes.DocumentUpdate,
	) ([]sharedTypes.DocumentUpdate, []sharedTypes.DocumentUpdate, error)

	ProcessUpdates(
		ctx context.Context,
		docId edgedb.UUID,
		doc *types.Doc,
		updates, transformUpdatesCache []sharedTypes.DocumentUpdate,
	) ([]sharedTypes.DocumentUpdate, []sharedTypes.DocumentUpdate, error)
}

func New(rm redisManager.Manager, rtRm realTimeRedisManager.Manager) Manager {
	return &manager{
		rm:   rm,
		rtRm: rtRm,
	}
}

type manager struct {
	rm   redisManager.Manager
	rtRm realTimeRedisManager.Manager
}

func (m *manager) ProcessOutstandingUpdates(ctx context.Context, docId edgedb.UUID, doc *types.Doc, transformUpdatesCache []sharedTypes.DocumentUpdate) ([]sharedTypes.DocumentUpdate, []sharedTypes.DocumentUpdate, error) {
	updates, err := m.rtRm.GetPendingUpdatesForDoc(ctx, docId)
	if err != nil {
		return nil, nil, errors.Tag(err, "cannot get work")
	}
	return m.ProcessUpdates(ctx, docId, doc, updates, transformUpdatesCache)
}

func (m *manager) ProcessUpdates(ctx context.Context, docId edgedb.UUID, doc *types.Doc, updates, transformUpdatesCache []sharedTypes.DocumentUpdate) ([]sharedTypes.DocumentUpdate, []sharedTypes.DocumentUpdate, error) {
	if len(updates) == 0 {
		return nil, nil, nil
	}
	var err error

	minVersion := updates[0].Version
	for _, update := range updates {
		if err = update.CheckVersion(doc.Version); err != nil {
			return nil, nil, err
		}
		if update.Version < minVersion {
			minVersion = update.Version
		}
	}
	maxVersion := doc.Version
	if len(transformUpdatesCache) != 0 {
		maxVersion = transformUpdatesCache[0].Version
	}

	if minVersion < maxVersion {
		missingTransformUpdates, err2 := m.rm.GetPreviousDocUpdatesUnderLock(
			ctx, docId, minVersion, maxVersion, doc.Version,
		)
		if err2 != nil {
			return nil, nil, errors.Tag(err2, "cannot get transform updates")
		}
		transformUpdatesCache = append(
			missingTransformUpdates, transformUpdatesCache...,
		)
	}

	processed := make([]sharedTypes.DocumentUpdate, 0)
	var s sharedTypes.Snapshot

outer:
	for _, update := range updates {
		incomingVersion := update.Version
		offset := len(transformUpdatesCache) - int(doc.Version-update.Version)
		for _, transformUpdate := range transformUpdatesCache[offset:] {
			if update.DupIfSource.Contains(transformUpdate.Meta.Source) {
				update.Dup = true
				processed = append(processed, update)
				continue outer
			}

			update.Op, err = text.Transform(update.Op, transformUpdate.Op)
			if err != nil {
				return processed, nil, err
			}
			update.Version++
		}

		s, err = text.Apply(doc.Snapshot, update.Op)
		if err != nil {
			return processed, nil, err
		}

		if err = s.CheckSize(); err != nil {
			return processed, nil, err
		}
		if incomingVersion == doc.Version && len(update.Hash) != 0 {
			if err = s.Hash().CheckMatches(update.Hash); err != nil {
				return processed, nil, err
			}
		}

		doc.Snapshot = s
		doc.Version++
		now := time.Now()
		doc.LastUpdatedCtx.At = now
		doc.LastUpdatedCtx.By = update.Meta.UserId
		update.Meta.Timestamp = sharedTypes.Timestamp(now.UnixMilli())
		processed = append(processed, update)
		transformUpdatesCache = append(transformUpdatesCache, update)
	}
	return processed, transformUpdatesCache, nil
}
