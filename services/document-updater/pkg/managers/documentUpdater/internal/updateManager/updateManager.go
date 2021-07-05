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
	) ([]types.DocumentUpdate, error)
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

func (m *manager) ProcessOutstandingUpdates(ctx context.Context, docId primitive.ObjectID, doc *types.Doc) ([]types.DocumentUpdate, error) {
	updates, err := m.rtRm.GetPendingUpdatesForDoc(ctx, docId)
	if err != nil {
		return nil, errors.Tag(err, "cannot get work")
	}
	if len(updates) == 0 {
		return nil, nil
	}

	minVersion := updates[0].Version
	for _, update := range updates {
		if err = update.CheckVersion(doc.Version); err != nil {
			return nil, err
		}
		if update.Version < minVersion {
			minVersion = update.Version
		}
	}
	allTransformUpdates, err := m.rm.GetPreviousDocUpdatesUnderLock(
		ctx, docId, minVersion, doc.Version,
	)
	if err != nil {
		return nil, errors.Tag(err, "cannot get transform updates")
	}

	processed := make([]types.DocumentUpdate, 0)
	var s types.Snapshot

outer:
	for _, update := range updates {
		offset := len(allTransformUpdates) - int(doc.Version-update.Version)
		transformUpdates := allTransformUpdates[offset:]
		for _, transformUpdate := range transformUpdates {
			if update.DupIfSource.Contains(transformUpdate.Meta.Source) {
				update.Dup = true
				processed = append(processed, update)
				continue outer
			}

			update.Op, err = text.Transform(update.Op, transformUpdate.Op)
			if err != nil {
				return processed, err
			}
			update.Version++
		}

		s, err = text.Apply(doc.Snapshot, update.Op)
		if err != nil {
			return processed, err
		}
		doc.Snapshot = s
		doc.Version++
		doc.LastUpdatedCtx.At = time.Now().Unix()
		doc.LastUpdatedCtx.By = update.Meta.UserId
		processed = append(processed, update)
		allTransformUpdates = append(allTransformUpdates, update)
	}
	return processed, nil
}
