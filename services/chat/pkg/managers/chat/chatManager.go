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
	) ([]*types.Message, error)

	SendGlobalMessage(
		ctx context.Context,
		projectId edgedb.UUID,
		content string,
		userId edgedb.UUID,
	) (*types.Message, error)

	SendThreadMessage(
		ctx context.Context,
		projectId, threadId edgedb.UUID,
		content string,
		userId edgedb.UUID,
	) (*types.Message, error)

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

func New(db *mongo.Database) Manager {
	return &manager{
		mm: message.New(db),
		tm: thread.NewThreadManager(db),
	}
}

type manager struct {
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
) ([]*types.Message, error) {
	room, err := m.tm.FindOrCreateThread(
		ctx,
		projectId,
		nil,
	)
	if err != nil {
		return nil, err
	}
	return m.mm.GetMessages(
		ctx,
		room.Id,
		limit,
		before,
	)
}

func (m *manager) SendGlobalMessage(
	ctx context.Context,
	projectId edgedb.UUID,
	content string,
	userId edgedb.UUID,
) (*types.Message, error) {
	return m.sendMessage(ctx, projectId, nil, content, userId)
}

func (m *manager) SendThreadMessage(
	ctx context.Context,
	projectId, threadId edgedb.UUID,
	content string,
	userId edgedb.UUID,
) (*types.Message, error) {
	return m.sendMessage(ctx, projectId, &threadId, content, userId)
}

var (
	NoContentProvided = &errors.ValidationError{Msg: "no content provided"}
	MaxMessageLength  = 10 * 1024
	ContentTooLong    = &errors.ValidationError{
		Msg: "content too long (> 10240 bytes)",
	}
)

func (m *manager) sendMessage(
	ctx context.Context,
	projectId edgedb.UUID,
	threadId *edgedb.UUID,
	content string,
	userId edgedb.UUID,
) (*types.Message, error) {
	if content == "" {
		return nil, NoContentProvided
	}
	if len(content) > MaxMessageLength {
		return nil, ContentTooLong
	}
	room, err := m.tm.FindOrCreateThread(
		ctx,
		projectId,
		threadId,
	)
	if err != nil {
		return nil, err
	}
	msg, err := m.mm.CreateMessage(
		ctx,
		room.Id,
		userId,
		content,
		nowMS(),
	)
	if err != nil {
		return nil, err
	}
	// sync with NodeJS implementation and shadow actual room_id
	msg.RoomId = projectId
	return msg, err
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
