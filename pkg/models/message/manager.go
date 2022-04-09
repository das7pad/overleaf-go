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

package message

import (
	"context"
	"time"

	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Manager interface {
	GetGlobalMessages(
		ctx context.Context,
		projectId edgedb.UUID,
		limit int64,
		before sharedTypes.Timestamp,
		target *[]Message,
	) error

	SendGlobalMessage(
		ctx context.Context,
		projectId edgedb.UUID,
		msg *Message,
	) error
}

func New(c *edgedb.Client) Manager {
	return &manager{
		c: c,
	}
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

func (m *manager) GetGlobalMessages(
	ctx context.Context,
	projectId edgedb.UUID,
	limit int64,
	before sharedTypes.Timestamp,
	messages *[]Message,
) error {
	var t time.Time
	if before == 0 {
		t = time.Now()
	} else {
		t = before.ToTime()
	}
	return rewriteEdgedbError(m.c.Query(ctx, `
select (select Project filter .id = <uuid>$0).chat.messages {
	id,
	content,
	created_at,
	user: { email: { email }, id, first_name, last_name },
}
filter .created_at < <datetime>$1
order by .created_at desc
limit <int64>$2
`, messages, projectId, t, limit))
}

func (m *manager) SendGlobalMessage(
	ctx context.Context,
	projectId edgedb.UUID,
	msg *Message,
) error {
	if err := checkContent(msg.Content); err != nil {
		return err
	}
	return rewriteEdgedbError(m.c.QuerySingle(ctx, `
select (insert Message {
	room := (select Project filter .id = <uuid>$0).chat,
	user := (select User filter .id = <uuid>$1),
	content := <str>$2,
}) {
	id,
	content,
	created_at,
	user: { email: { email }, id, first_name, last_name },
}
`, msg, projectId, msg.User.Id, msg.Content))
}

var (
	NoContentProvided = &errors.ValidationError{Msg: "no content provided"}
	MaxMessageLength  = 10 * 1024
	ContentTooLong    = &errors.ValidationError{
		Msg: "content too long (> 10240 bytes)",
	}
)

func checkContent(content string) error {
	if content == "" {
		return NoContentProvided
	}
	if len(content) > MaxMessageLength {
		return ContentTooLong
	}
	return nil
}
