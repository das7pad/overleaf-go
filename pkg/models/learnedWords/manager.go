// Golang port of Overleaf
// Copyright (C) 2021 Jakob Ackermann <das7pad@outlook.com>
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

package learnedWords

import (
	"context"

	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/user"
)

type Manager interface {
	DeleteDictionary(
		ctx context.Context,
		userId edgedb.UUID,
	) error

	GetDictionary(
		ctx context.Context,
		userId edgedb.UUID,
	) ([]string, error)

	LearnWord(
		ctx context.Context,
		userId edgedb.UUID,
		word string,
	) error

	UnlearnWord(
		ctx context.Context,
		userId edgedb.UUID,
		word string,
	) error
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

func (m manager) DeleteDictionary(ctx context.Context, userId edgedb.UUID) error {
	return m.c.QuerySingle(
		ctx,
		"update User filter .id = <uuid>$0 set { learned_words := {} }",
		&user.IdField{},
		userId,
	)
}

func (m manager) GetDictionary(ctx context.Context, userId edgedb.UUID) ([]string, error) {
	var preference spellingPreference
	err := m.c.QuerySingle(
		ctx,
		"select User { learned_words } filter .id = <uuid>$0",
		&preference,
		userId,
	)
	if err != nil {
		return nil, rewriteEdgedbError(err)
	}
	return preference.LearnedWords, nil
}

func (m manager) LearnWord(ctx context.Context, userId edgedb.UUID, word string) error {
	err := m.c.QuerySingle(
		ctx,
		`\
update User
filter .id = <uuid>$0 and <str>$1 not in .learned_words
set { learned_words += <str>$1 }`,
		&user.IdField{},
		userId, word,
	)
	if err != nil {
		err = rewriteEdgedbError(err)
		if errors.IsNotFoundError(err) {
			return nil
		}
		return err
	}
	return nil
}

func (m manager) UnlearnWord(ctx context.Context, userId edgedb.UUID, word string) error {
	err := m.c.QuerySingle(
		ctx,
		`\
update User
filter .id = <uuid>$0
set { learned_words -= <str>$1 }`,
		&user.IdField{},
		userId, word,
	)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	return nil
}
