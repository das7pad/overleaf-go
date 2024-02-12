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

package trackChanges

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/das7pad/overleaf-go/pkg/models/docHistory"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater"
	"github.com/das7pad/overleaf-go/services/track-changes/pkg/managers/trackChanges/diff"
	"github.com/das7pad/overleaf-go/services/track-changes/pkg/managers/trackChanges/flush"
	"github.com/das7pad/overleaf-go/services/track-changes/pkg/managers/trackChanges/updates"
)

type Manager interface {
	diffManager
	flushManager
	updatesManager
}

func New(db *pgxpool.Pool, client redis.UniversalClient, dum documentUpdater.Manager) (Manager, error) {
	fm, err := flush.New(db, client)
	if err != nil {
		return nil, err
	}
	dhm := docHistory.New(db)
	dfm := diff.New(dhm, fm, dum)
	um := updates.New(dhm, fm)
	return &manager{
		diffManager:    dfm,
		flushManager:   fm,
		updatesManager: um,
	}, nil
}

type diffManager = diff.Manager

type flushManager = flush.Manager

type updatesManager = updates.Manager

type manager struct {
	diffManager
	flushManager
	updatesManager
}
