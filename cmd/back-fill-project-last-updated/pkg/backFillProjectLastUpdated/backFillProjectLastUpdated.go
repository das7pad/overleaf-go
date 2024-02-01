// Golang port of Overleaf
// Copyright (C) 2024 Jakob Ackermann <das7pad@outlook.com>
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

package backFillProjectLastUpdated

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

func Run(ctx context.Context, db *pgxpool.Pool) error {
	r, err := db.Query(
		ctx, "SELECT id FROM projects WHERE last_updated_by IS NULL",
	)
	if err != nil {
		return errors.Tag(err, "find projects")
	}

	batch := pgx.Batch{}
	var id sharedTypes.UUID
	for r.Next() {
		if err = r.Scan(&id); err != nil {
			return errors.Tag(err, "scan id")
		}
		batch.Queue(`
WITH new_last_updated
         AS ((SELECT user_id AS last_updated_by, end_at AS last_updated_at
              FROM doc_history dh
                       INNER JOIN tree_nodes t ON t.id = dh.doc_id
              WHERE t.project_id = $1
              ORDER BY dh.end_at DESC
              LIMIT 1)
             UNION ALL
             (SELECT owner_id AS last_updated_by, last_updated_at
              FROM projects
              WHERE id = $1)
             LIMIT 1)
UPDATE projects p
SET last_updated_by = new_last_updated.last_updated_by,
    last_updated_at = new_last_updated.last_updated_at
FROM new_last_updated
WHERE p.id = $1
  AND p.last_updated_by IS NULL
`, id)
		if batch.Len() > 1_000 {
			if _, err = db.SendBatch(ctx, &batch).Exec(); err != nil {
				return errors.Tag(err, "flush batch")
			}
			batch = pgx.Batch{}
		}
	}
	if err = r.Err(); err != nil {
		return errors.Tag(err, "iter projects")
	}
	if batch.Len() > 0 {
		if _, err = db.SendBatch(ctx, &batch).Exec(); err != nil {
			return errors.Tag(err, "flush batch")
		}
	}
	return nil
}
