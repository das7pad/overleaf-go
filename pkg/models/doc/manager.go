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
	"database/sql"

	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Manager interface {
	UpdateDoc(ctx context.Context, projectId, docId sharedTypes.UUID, update *ForDocUpdate) error
}

func New(db *sql.DB) Manager {
	return &manager{db: db}
}

func rewriteEdgedbError(err error) error {
	if e, ok := err.(edgedb.Error); ok && e.Category(edgedb.NoDataError) {
		return &errors.NotFoundError{}
	}
	return err
}

type manager struct {
	c  *edgedb.Client
	db *sql.DB
}

func (m *manager) UpdateDoc(ctx context.Context, projectId, docId sharedTypes.UUID, update *ForDocUpdate) error {
	if err := update.Snapshot.Validate(); err != nil {
		return err
	}

	ids := make([]sharedTypes.UUID, 2)
	err := m.c.Query(ctx, `
with
	d := (select Doc filter .id = <uuid>$0 and .project.id = <uuid>$1),
	p := (
		update d.project
		filter .last_updated_at < <datetime>$2
		set {
			last_updated_at := <datetime>$2,
			last_updated_by := (select User filter .id = <uuid>$3),
		}
	),
	updatedDoc := (
		update d
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
