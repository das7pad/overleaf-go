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

	"github.com/edgedb/edgedb-go"
)

type Manager interface {
	Create(ctx context.Context, content string) error
	DeleteAll(ctx context.Context) error
	GetAll(ctx context.Context) ([]Full, error)
}

func New(c *edgedb.Client) Manager {
	return &manager{
		c: c,
	}
}

type manager struct {
	c *edgedb.Client
}

func (m *manager) Create(ctx context.Context, content string) error {
	return m.c.QuerySingle(
		ctx,
		"insert SystemMessage{ content := <str>$0 }",
		&IdField{},
		content,
	)
}

func (m *manager) DeleteAll(ctx context.Context) error {
	return m.c.Execute(ctx, "delete SystemMessage")
}

func (m *manager) GetAll(ctx context.Context) ([]Full, error) {
	messages := make([]Full, 0)
	return messages, m.c.Query(
		ctx,
		"select SystemMessage{ id, content }",
		&messages,
	)
}
