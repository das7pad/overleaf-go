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

package updates

import (
	"context"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/docHistory"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/track-changes/pkg/managers/trackChanges/flush"
	"github.com/das7pad/overleaf-go/services/track-changes/pkg/types"
)

type Manager interface {
	GetProjectHistoryUpdates(ctx context.Context, request *types.GetProjectHistoryUpdatesRequest, response *types.GetProjectHistoryUpdatesResponse) error
}

func New(dhm docHistory.Manager, fm flush.Manager) Manager {
	return &manager{
		dhm: dhm,
		fm:  fm,
	}
}

const (
	fetchAtMostNUpdates   = 30
	returnAtLeastNUpdates = 15

	mergeWindow = sharedTypes.Timestamp(5 * time.Minute / time.Millisecond)
)

type manager struct {
	dhm docHistory.Manager
	fm  flush.Manager
}

func (m *manager) GetProjectHistoryUpdates(ctx context.Context, r *types.GetProjectHistoryUpdatesRequest, res *types.GetProjectHistoryUpdatesResponse) error {
	if err := m.fm.FlushProject(ctx, r.ProjectId); err != nil {
		return errors.Tag(err, "flush project")
	}

	before := time.UnixMilli(int64(r.Before))
	batch := docHistory.GetForProjectResult{
		History: make([]docHistory.ProjectUpdate, 0, fetchAtMostNUpdates),
		Users:   make([]user.WithPublicInfo, 0, fetchAtMostNUpdates),
	}
	var lastRawUpdateHasBigDelete bool
	var lastUpdateEndAt sharedTypes.Timestamp
	for len(res.Updates) < returnAtLeastNUpdates {
		{
			err := m.dhm.GetForProject(ctx, r.ProjectId, before, &batch)
			if err != nil {
				return errors.Tag(err, "cannot get next batch of history")
			}
		}
		h := batch.History
		if len(batch.History) == fetchAtMostNUpdates {
			before = h[fetchAtMostNUpdates-1].EndAt
			res.NextBeforeTimestamp = sharedTypes.Timestamp(before.UnixMilli())
		} else {
			res.NextBeforeTimestamp = 0
		}
		if len(h) == 0 {
			return nil
		}

		for _, update := range h {
			docId := update.Doc.Id.String()
			startAt := sharedTypes.Timestamp(update.StartAt.UnixMilli())
			endAt := sharedTypes.Timestamp(update.EndAt.UnixMilli())
			if len(res.Updates) == 0 ||
				lastRawUpdateHasBigDelete ||
				lastUpdateEndAt-startAt > mergeWindow {
				res.Updates = append(res.Updates, types.Update{
					Meta: types.DocUpdateMeta{
						Users: []user.WithPublicInfoAndNonStandardId{
							batch.Users.GetUserNonStandardId(update.User.Id),
						},
						StartTs: startAt,
						EndTs:   endAt,
					},
					Docs: map[string]types.DocUpdateBounds{
						docId: {
							FromV: update.Version,
							ToV:   update.Version,
						},
					},
				})
				lastRawUpdateHasBigDelete = update.HasBigDelete
				lastUpdateEndAt = endAt
				continue
			}
			lastUpdate := &res.Updates[len(res.Updates)-1]
			if d, exists := lastUpdate.Docs[docId]; exists {
				lastUpdate.Docs[docId] = types.DocUpdateBounds{
					FromV: update.Version,
					ToV:   d.ToV,
				}
			} else {
				lastUpdate.Docs[docId] = types.DocUpdateBounds{
					FromV: update.Version,
					ToV:   update.Version,
				}
			}
			alreadyAdded := false
			for _, u := range lastUpdate.Meta.Users {
				if u.Id == update.User.Id {
					alreadyAdded = true
					break
				}
			}
			if !alreadyAdded {
				lastUpdate.Meta.Users = append(
					lastUpdate.Meta.Users,
					batch.Users.GetUserNonStandardId(update.User.Id),
				)
			}
			if lastUpdate.Meta.StartTs.ToTime().After(update.StartAt) {
				lastUpdate.Meta.StartTs = startAt
			}
			lastRawUpdateHasBigDelete = update.HasBigDelete
			lastUpdateEndAt = lastUpdate.Meta.EndTs
		}
	}
	return nil
}
