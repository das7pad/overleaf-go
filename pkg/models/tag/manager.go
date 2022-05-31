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

	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/errors"
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
	c  *edgedb.Client
	db *sql.DB
}

func rewriteEdgedbError(err error) error {
	if e, ok := err.(edgedb.Error); ok && e.Category(edgedb.NoDataError) {
		return &errors.NotFoundError{}
	}
	return err
}

func (m *manager) AddProject(ctx context.Context, userId, tagId, projectId sharedTypes.UUID) error {
	err := m.c.QuerySingle(
		ctx,
		`
with
	u := (select User filter .id = <uuid>$1 and not exists .deleted_at),
	p := (select Project filter .id = <uuid>$2 and not exists .deleted_at),
	pWithAuth := (select p filter u in .min_access_ro)
update Tag
filter .id = <uuid>$0 and .user = u
set { projects += pWithAuth }`,
		&IdField{},
		tagId, userId, projectId,
	)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	return nil
}

func (m *manager) Delete(ctx context.Context, userId, tagId sharedTypes.UUID) error {
	err := m.c.QuerySingle(
		ctx,
		`
with u := (select User filter .id = <uuid>$0 and not exists .deleted_at)
delete Tag
filter .id = <uuid>$0 and .user = u`,
		&IdField{},
		tagId, userId,
	)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	return nil
}

func (m *manager) EnsureExists(ctx context.Context, userId sharedTypes.UUID, name string) (*Full, error) {
	t := &Full{}
	err := m.c.QuerySingle(
		ctx,
		`
with u := (select User filter .id = <uuid>$0 and not exists .deleted_at)
insert Tag { name := <str>$1, user := u }
unless conflict on (.name, .user)
else (select Tag { id, projects } filter .name = <str>$1 and .user = u)`,
		t,
		userId, name,
	)
	if err != nil {
		return nil, rewriteEdgedbError(err)
	}
	t.Name = name
	t.ProjectIds = make([]sharedTypes.UUID, len(t.Projects))
	return t, nil
}

func (m *manager) RemoveProjectFromTag(ctx context.Context, userId, tagId, projectId sharedTypes.UUID) error {
	err := m.c.QuerySingle(
		ctx,
		`
with u := (select User filter .id = <uuid>$0 and not exists .deleted_at)
update Tag
filter .id = <uuid>$0 and .user = u
set { projects -= (select Project filter .id = <uuid>$2) }`,
		&IdField{},
		tagId, userId, projectId,
	)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	return nil
}

func (m *manager) Rename(ctx context.Context, userId, tagId sharedTypes.UUID, newName string) error {
	err := m.c.QuerySingle(
		ctx,
		`
with u := (select User filter .id = <uuid>$0 and not exists .deleted_at)
update Tag
filter .id = <uuid>$0 and .user = u
set { name := <str>$2 }`,
		&IdField{},
		tagId, userId, newName,
	)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	return nil
}
