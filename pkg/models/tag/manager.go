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

package tag

import (
	"context"
	"database/sql"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Manager interface {
	AddProject(ctx context.Context, userId, tagId, projectId sharedTypes.UUID) error
	Delete(ctx context.Context, userId, tagId sharedTypes.UUID) error
	EnsureExists(ctx context.Context, userId sharedTypes.UUID, name string) (*Full, error)
	RemoveProjectFromTag(ctx context.Context, userId, tagId, projectId sharedTypes.UUID) error
	Rename(ctx context.Context, userId, tagId sharedTypes.UUID, newName string) error
}

func New(db *sql.DB) Manager {
	return &manager{db: db}
}

type manager struct {
	db *sql.DB
}

func getErr(_ sql.Result, err error) error {
	return err
}

func (m *manager) AddProject(ctx context.Context, userId, tagId, projectId sharedTypes.UUID) error {
	return getErr(m.db.ExecContext(ctx, `
WITH project AS (SELECT projects.id
                 FROM projects
                          LEFT JOIN project_members pm
                                    on projects.id = pm.project_id
                 WHERE projects.id = $3
                   AND projects.deleted_at IS NULL
                   AND (projects.owner_id = $2 OR pm.user_id = $2)
                 LIMIT 1),
     tag AS (SELECT id FROM tags WHERE id = $1 AND user_id = $2)
INSERT
INTO tag_entries (project_id, tag_id)
SELECT project.id, tag.id
FROM project,
     tag
ON CONFLICT DO NOTHING
`, tagId, userId, projectId))
}

func (m *manager) Delete(ctx context.Context, userId, tagId sharedTypes.UUID) error {
	return getErr(m.db.ExecContext(ctx, `
DELETE
FROM tags
WHERE id = $1
  AND user_id = $2
`, tagId, userId))
}

func (m *manager) EnsureExists(ctx context.Context, userId sharedTypes.UUID, name string) (*Full, error) {
	t := &Full{}
	err := m.db.QueryRowContext(ctx, `
INSERT INTO tags (id, name, user_id)
VALUES (gen_random_uuid(), $2, $1)
-- Perform a no-op update to get the id back.
-- Use the user_id, which is static, whereas the name is not.
ON CONFLICT DO UPDATE SET user_id = user_id
RETURNING id
`, userId, name).Scan(&t.Id)
	if err != nil {
		return nil, err
	}
	t.ProjectIds = make([]sharedTypes.UUID, 0)
	return t, nil
}

func (m *manager) RemoveProjectFromTag(ctx context.Context, userId, tagId, projectId sharedTypes.UUID) error {
	return getErr(m.db.ExecContext(ctx, `
WITH tag AS (SELECT id FROM tags WHERE id = $1 AND user_id = $2)
DELETE
FROM tag_entries
WHERE tag_id = tag.id
  AND project_id = $3
`, tagId, userId, projectId))
}

func (m *manager) Rename(ctx context.Context, userId, tagId sharedTypes.UUID, newName string) error {
	return getErr(m.db.ExecContext(ctx, `
UPDATE tags
SET name = $3
WHERE id = $1
  AND user_id = $2
`, tagId, userId, newName))
}
