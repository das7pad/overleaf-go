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

package systemMessage

import (
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
)

type Manager interface {
	Create(ctx context.Context, content string) error
	DeleteAll(ctx context.Context) error
	GetAll(ctx context.Context) ([]Full, error)
}

func New(db *pgxpool.Pool) Manager {
	return &manager{db: db}
}

type manager struct {
	db *pgxpool.Pool
}

func (m *manager) Create(ctx context.Context, content string) error {
	_, err := m.db.Exec(
		ctx,
		`
INSERT INTO system_messages (content, id)
VALUES ($1, gen_random_uuid())
`, content)
	return err
}

func (m *manager) DeleteAll(ctx context.Context) error {
	_, err := m.db.Exec(ctx, `
DELETE FROM system_messages
WHERE TRUE
`)
	return err
}

func (m *manager) GetAll(ctx context.Context) ([]Full, error) {
	r, err := m.db.Query(
		ctx,
		`
SELECT id, content FROM system_messages
`)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	out := make([]Full, 0)
	for r.Next() {
		out = append(out, Full{})
		i := len(out) - 1
		if err = r.Scan(&out[i].Id, &out[i].Content); err != nil {
			return nil, err
		}
	}
	if err = r.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
