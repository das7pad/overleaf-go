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

package docHistory

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Manager interface {
	InsertBulk(ctx context.Context, projectId, docId edgedb.UUID, dh []ForInsert) error
	GetLastVersion(ctx context.Context, projectId, docId edgedb.UUID) (sharedTypes.Version, error)
	GetForDoc(ctx context.Context, projectId, docId edgedb.UUID, from, to sharedTypes.Version, h *[]DocHistory) error
	GetForProject(ctx context.Context, projectId edgedb.UUID, before time.Time, r *GetForProjectResult) error
}

func New(c *edgedb.Client) Manager {
	return &manager{c: c}
}

func rewriteEdgedbError(err error) error {
	if err == nil {
		return nil
	}
	if e, ok := err.(edgedb.Error); ok && e.Category(edgedb.NoDataError) {
		return &errors.NotFoundError{}
	}
	return err
}

type manager struct {
	c *edgedb.Client
}

func (m *manager) InsertBulk(ctx context.Context, projectId, docId edgedb.UUID, dh []ForInsert) error {
	for i := 0; i < len(dh); i++ {
		blob, err := json.Marshal(dh[i].Op)
		if err != nil {
			return errors.Tag(
				err,
				fmt.Sprintf("cannot serialize op for update %d", i),
			)
		}
		dh[i].OpForDB = blob
	}
	blob, err := json.Marshal(dh)
	if err != nil {
		return errors.Tag(err, "cannot serialize history for db insert")
	}

	ok := false
	return m.c.QuerySingle(ctx, `
with
	d := (select Doc filter .id = <uuid>$0 and .project.id = <uuid>$1),
	inserted := (
		for elem in json_array_unpack(<json>$2) union (
			insert DocHistory {
				doc := d,
				user := (select User filter .id = <uuid>elem['user_id']),
				version := <int64>elem['version'],
				start_at := <datetime>elem['start_at'],
				end_at := <datetime>elem['end_at'],
				op := <json>elem['op'],
				has_big_delete := <bool>elem['has_big_delete'],
			}
		)
	),
select {exists inserted}
`,
		&ok,
		projectId,
		docId,
		blob,
	)
}

func (m *manager) GetLastVersion(ctx context.Context, projectId, docId edgedb.UUID) (sharedTypes.Version, error) {
	var v sharedTypes.Version
	err := m.c.QuerySingle(ctx, `
select DocHistory.version
filter .doc.id = <uuid>$0 and .doc.project.id = <uuid>$1
order by .version desc
limit 1
`, &v, projectId, docId)
	if err != nil {
		err = rewriteEdgedbError(err)
		if errors.IsNotFoundError(err) {
			return 0, nil
		}
		return 0, err
	}
	return v, nil
}

func (m *manager) GetForDoc(ctx context.Context, projectId, docId edgedb.UUID, from, to sharedTypes.Version, h *[]DocHistory) error {
	return m.c.Query(ctx, `
with
	doc := (select Doc filter .id = <uuid>$0 and .project.id = <uuid>$1),
select DocHistory {
	version,
	start_at,
	end_at,
	op,
}
filter
	.doc = doc
and .version > <int64>$2
and .version < <int64>$3
`,
		h,
		docId,
		projectId,
		from,
		to,
	)
}

type GetForProjectResult struct {
	History []ProjectUpdate  `edgedb:"history"`
	Users   user.BulkFetched `edgedb:"users"`
}

func (m *manager) GetForProject(ctx context.Context, projectId edgedb.UUID, before time.Time, r *GetForProjectResult) error {
	return m.c.Query(ctx, `
with
	hIn := (
		select DocHistory
		filter
			.doc.project.id = <uuid>$0
		and .end_at < <datetime>$1
		order by .end_at desc
		limit <int64>$2
	),
	hEnd := (
		select hIn.end_at
		order by .end_at asc
		limit 1
	),
	h := (
		select DocHistory
		filter
			.doc.project.id = <uuid>$0
		and .end_at < <datetime>$1
		and .end_at >= hEnd
		order by .end_at desc
	),
	users := (select distinct(h.user))
select {
	history := h {
		version,
		start_at,
		end_at,
		has_big_delete,
	},
	users := users { id, email: { email }, first_name, last_name },
}
`,
		r,
		projectId,
		before,
		cap(r.History),
	)
}
