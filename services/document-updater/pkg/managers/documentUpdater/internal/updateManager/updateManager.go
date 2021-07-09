// Golang port of the Overleaf document-updater service
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

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/document-updater/pkg/errors"
	"github.com/das7pad/document-updater/pkg/managers/documentUpdater/internal/realTimeRedisManager"
	"github.com/das7pad/document-updater/pkg/managers/documentUpdater/internal/redisManager"
	"github.com/das7pad/document-updater/pkg/sharejs/types/text"
	"github.com/das7pad/document-updater/pkg/types"
)

type Manager interface {
	ProcessOutstandingUpdates(
		ctx context.Context,
		docId primitive.ObjectID,
		doc *types.Doc,
		transformUpdatesCache []types.DocumentUpdate,
	) ([]types.DocumentUpdate, []types.DocumentUpdate, error)
}

func New(rm redisManager.Manager, rtRm realTimeRedisManager.Manager) Manager {
	return &manager{
		rm:   rm,
		rtRm: rtRm,
	}
}

const (
	maxDocLength = 2 * 1024 * 1024
)

type manager struct {
	rm   redisManager.Manager
	rtRm realTimeRedisManager.Manager
}

func (m *manager) ProcessOutstandingUpdates(ctx context.Context, docId primitive.ObjectID, doc *types.Doc, transformUpdatesCache []types.DocumentUpdate) ([]types.DocumentUpdate, []types.DocumentUpdate, error) {
	updates, err := m.rtRm.GetPendingUpdatesForDoc(ctx, docId)
	if err != nil {
		return nil, nil, errors.Tag(err, "cannot get work")
	}
	if len(updates) == 0 {
		return nil, nil, nil
	}

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

	processed := make([]types.DocumentUpdate, 0)
	var s types.Snapshot

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

		if len(s) > maxDocLength {
			return processed, nil, &errors.CodedError{
				Description: "Update takes doc over max doc size",
			}
		}
		if incomingVersion == doc.Version && len(update.Hash) != 0 {
			if err = s.Hash().CheckMatches(update.Hash); err != nil {
				return processed, nil, err
			}
		}

		doc.Snapshot = s
		doc.Version++
		doc.LastUpdatedCtx.At = time.Now().Unix()
		doc.LastUpdatedCtx.By = update.Meta.UserId
		processed = append(processed, update)
		transformUpdatesCache = append(transformUpdatesCache, update)
	}
	return processed, transformUpdatesCache, nil
}
