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

package chat

import (
	"context"
	"time"

	"github.com/edgedb/edgedb-go"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/chat/pkg/managers/chat/internal/message"
	"github.com/das7pad/overleaf-go/services/chat/pkg/managers/chat/internal/thread"
	"github.com/das7pad/overleaf-go/services/chat/pkg/types"
)

type Manager interface {
	GetGlobalMessages(
		ctx context.Context,
		projectId edgedb.UUID,
		limit int64,
		before sharedTypes.Timestamp,
		target *[]types.Message,
	) error

	SendGlobalMessage(
		ctx context.Context,
		projectId edgedb.UUID,
		msg *types.Message,
	) error

	SendThreadMessage(
		ctx context.Context,
		projectId, threadId edgedb.UUID,
		msg *types.Message,
	) error

	GetAllThreads(
		ctx context.Context,
		projectId edgedb.UUID,
	) (types.Threads, error)

	ResolveThread(
		ctx context.Context,
		projectId, threadId, userId edgedb.UUID,
	) error

	ReopenThread(
		ctx context.Context,
		projectId, threadId edgedb.UUID,
	) error

	DeleteThread(
		ctx context.Context,
		projectId, threadId edgedb.UUID,
	) error

	EditMessage(
		ctx context.Context,
		projectId, threadId, messageId edgedb.UUID,
		content string,
	) error

	DeleteMessage(
		ctx context.Context,
		projectId, threadId, messageId edgedb.UUID,
	) error

	DeleteProject(ctx context.Context, projectId edgedb.UUID) error
}

func New(c *edgedb.Client, db *mongo.Database) Manager {
	return &manager{
		c:  c,
		mm: message.New(db),
		tm: thread.NewThreadManager(db),
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
	c  *edgedb.Client
	mm message.Manager
	tm thread.Manager
}

func (m *manager) DeleteProject(ctx context.Context, projectId edgedb.UUID) error {
	chatThread, err := m.tm.FindOrCreateThread(ctx, projectId, nil)
	if err != nil {
		return errors.Tag(err, "cannot get chat thread")
	}
	threads, err := m.tm.FindAllThreadRooms(ctx, projectId)
	if err != nil {
		return errors.Tag(err, "cannot get review threads")
	}
	roomIds := make([]edgedb.UUID, len(threads)+1)
	for i, room := range threads {
		roomIds[i] = room.Id
	}
	roomIds[len(roomIds)-1] = chatThread.Id
	if err = m.mm.DeleteProjectMessages(ctx, roomIds); err != nil {
		return errors.Tag(err, "cannot delete messages")
	}
	if err = m.tm.DeleteProjectThreads(ctx, projectId); err != nil {
		return errors.Tag(err, "cannot delete threads")
	}
	return nil
}

func nowMS() float64 {
	return float64(time.Now().UnixNano() / 1e6)
}

func (m *manager) GetGlobalMessages(
	ctx context.Context,
	projectId edgedb.UUID,
	limit int64,
	before sharedTypes.Timestamp,
	target *[]types.Message,
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
`, target, projectId, t, limit))
}

func (m *manager) SendGlobalMessage(
	ctx context.Context,
	projectId edgedb.UUID,
	msg *types.Message,
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

func (m *manager) SendThreadMessage(
	ctx context.Context,
	projectId, threadId edgedb.UUID,
	msg *types.Message,
) error {
	if err := checkContent(msg.Content); err != nil {
		return err
	}
	return rewriteEdgedbError(m.c.QuerySingle(ctx, `
select (insert Message {
	room := (
		select ReviewThread filter .id = <uuid>$0 and .project.id = <uuid>$1
	),
	user := (select User filter .id = <uuid>$2),
	content := <str>$3,
}) {
	id,
	content,
	created_at,
	user: { email: { email }, id, first_name, last_name },
}
`, msg, threadId, projectId, msg.User.Id, msg.Content))
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

func groupMessagesByThreads(rooms []thread.Room, messages []*types.Message) types.Threads {
	roomById := map[edgedb.UUID]thread.Room{}
	for _, room := range rooms {
		roomById[room.Id] = room
	}
	threads := make(types.Threads, len(rooms))
	for _, msg := range messages {
		room, exists := roomById[msg.RoomId]
		if !exists {
			continue
		}
		t, exists := threads[room.ThreadId.String()]
		if !exists {
			t = &types.Thread{
				Messages: make([]*types.Message, 0),
			}
			resolved := room.Resolved != nil
			if resolved {
				t.Resolved = &resolved
				t.ResolvedAt = &room.Resolved.At
				t.ResolvedByUserId = &room.Resolved.ByUserId
			}
		}
		t.Messages = append(t.Messages, msg)
		threads[room.ThreadId.String()] = t
	}
	return threads
}

func (m *manager) GetAllThreads(
	ctx context.Context,
	projectId edgedb.UUID,
) (types.Threads, error) {
	rooms, err := m.tm.FindAllThreadRooms(ctx, projectId)
	if err != nil {
		return nil, err
	}

	roomIds := make([]edgedb.UUID, len(rooms))
	for i, room := range rooms {
		roomIds[i] = room.Id
	}
	messages, err := m.mm.FindAllMessagesInRooms(ctx, roomIds)

	return groupMessagesByThreads(rooms, messages), nil
}

func (m *manager) ResolveThread(
	ctx context.Context,
	projectId, threadId, userId edgedb.UUID,
) error {
	return m.tm.ResolveThread(ctx, projectId, threadId, userId)
}

func (m *manager) ReopenThread(
	ctx context.Context,
	projectId, threadId edgedb.UUID,
) error {
	return m.tm.ReopenThread(ctx, projectId, threadId)
}

func (m *manager) DeleteThread(
	ctx context.Context,
	projectId, threadId edgedb.UUID,
) error {
	roomId, err := m.tm.DeleteThread(ctx, projectId, threadId)
	if err != nil {
		return err
	}
	return m.mm.DeleteAllMessagesInRoom(ctx, *roomId)
}

func (m *manager) EditMessage(
	ctx context.Context,
	projectId, threadId, messageId edgedb.UUID,
	content string,
) error {
	room, err := m.tm.FindOrCreateThread(
		ctx,
		projectId,
		&threadId,
	)
	if err != nil {
		return err
	}
	return m.mm.UpdateMessage(
		ctx,
		room.Id,
		messageId,
		content,
		nowMS(),
	)
}

func (m *manager) DeleteMessage(
	ctx context.Context,
	projectId, threadId, messageId edgedb.UUID,
) error {
	room, err := m.tm.FindOrCreateThread(
		ctx,
		projectId,
		&threadId,
	)
	if err != nil {
		return err
	}
	return m.mm.DeleteMessage(
		ctx,
		room.Id,
		messageId,
	)
}
