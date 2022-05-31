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

package history

import (
	"context"
	"database/sql"

	"github.com/go-redis/redis/v8"

	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater"
	"github.com/das7pad/overleaf-go/services/track-changes/pkg/managers/trackChanges"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	GetProjectHistoryUpdates(ctx context.Context, request *types.GetProjectHistoryUpdatesRequest, response *types.GetProjectHistoryUpdatesResponse) error
	GetDocDiff(ctx context.Context, request *types.GetDocDiffRequest, response *types.GetDocDiffResponse) error
	RestoreDocVersion(ctx context.Context, request *types.RestoreDocVersionRequest) error
}

func New(db *sql.DB, client redis.UniversalClient, dum documentUpdater.Manager) (Manager, error) {
	return trackChanges.New(db, client, dum)
}
