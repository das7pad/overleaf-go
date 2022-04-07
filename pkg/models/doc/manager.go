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

	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type Manager interface {
	UpdateDoc(ctx context.Context, projectId, docId edgedb.UUID, update *ForDocUpdate) error
}

func New(c *edgedb.Client) Manager {
	return &manager{
		c: c,
	}
}

func rewriteEdgedbError(err error) error {
	if e, ok := err.(edgedb.Error); ok && e.Category(edgedb.NoDataError) {
		return &errors.NotFoundError{}
	}
	return err
}

type manager struct {
	c *edgedb.Client
}

func (m *manager) UpdateDoc(ctx context.Context, projectId, docId edgedb.UUID, update *ForDocUpdate) error {
	if err := update.Snapshot.Validate(); err != nil {
		return err
	}

	ids := make([]edgedb.UUID, 2)
	err := m.c.Query(ctx, `
with
	d := (select Doc filter .id = <uuid>$0 and .project.id = <uuid>$1),
	p := (
		update Project
		filter Project = d.project and .last_updated_at < <datetime>$2
		set {
			last_updated_at := <datetime>$2,
			last_updated_by := (select User filter .id = <uuid>$3),
		}
	),
	updatedDoc := (
		update Doc
		filter Doc = d
		set {
			snapshot := <str>$4,
			version := <int64>$5,
		}
	)
select {p.id,updatedDoc.id}
`,
		&ids,
		docId, projectId,
		update.LastUpdatedAt, update.LastUpdatedBy,
		string(update.Snapshot), int64(update.Version),
	)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	if len(ids) == 0 {
		return &errors.NotFoundError{}
	}
	return nil
}
