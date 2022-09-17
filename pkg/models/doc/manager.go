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

package doc

import (
	"context"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4/pgxpool"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Manager interface {
	UpdateDoc(ctx context.Context, projectId, docId sharedTypes.UUID, update *ForDocUpdate) error
}

func New(db *pgxpool.Pool) Manager {
	return &manager{db: db}
}

func getErr(_ pgconn.CommandTag, err error) error {
	return err
}

type manager struct {
	db *pgxpool.Pool
}

func (m *manager) UpdateDoc(ctx context.Context, projectId, docId sharedTypes.UUID, update *ForDocUpdate) error {
	if err := update.Snapshot.Validate(); err != nil {
		return err
	}

	return getErr(m.db.Exec(ctx, `
WITH d AS (
    UPDATE docs d
        SET snapshot = $3, version = $4
        FROM tree_nodes t
        WHERE d.id = $2 AND d.id = t.id AND t.project_id = $1
        RETURNING t.project_id)

UPDATE projects
SET last_updated_at = $5,
    last_updated_by = $6
FROM d
WHERE id = d.project_id
  AND last_updated_at < $5;
`,
		projectId, docId, string(update.Snapshot), int64(update.Version),
		update.LastUpdatedAt, update.LastUpdatedBy,
	))
}
