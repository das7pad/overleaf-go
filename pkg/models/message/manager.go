// Golang port of Overleaf
// Copyright (C) 2021-2023 Jakob Ackermann <das7pad@outlook.com>
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

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Manager interface {
	GetGlobalMessages(ctx context.Context, projectId sharedTypes.UUID, limit int64, before sharedTypes.Timestamp, target *[]Message) error
	SendGlobalMessage(ctx context.Context, projectId sharedTypes.UUID, msg *Message) error
}

func New(db *pgxpool.Pool) Manager {
	return &manager{db: db}
}

type manager struct {
	db *pgxpool.Pool
}

func (m *manager) GetGlobalMessages(ctx context.Context, projectId sharedTypes.UUID, limit int64, before sharedTypes.Timestamp, messages *[]Message) error {
	var t time.Time
	if before == 0 {
		t = time.Now()
	} else {
		t = before.ToTime()
	}
	r, err := m.db.Query(ctx, `
SELECT cm.id,
       cm.content,
       cm.created_at,
       coalesce(u.id, '00000000-0000-0000-0000-000000000000'::UUID),
       coalesce(u.email, ''),
       coalesce(u.first_name, ''),
       coalesce(u.last_name, '')
FROM chat_messages cm
         LEFT JOIN users u ON cm.user_id = u.id
WHERE cm.project_id = $1
  AND cm.created_at < $2
  AND u.deleted_at IS NULL
ORDER BY cm.created_at DESC
LIMIT $3
`, projectId, t, limit)
	if err != nil {
		return err
	}
	defer r.Close()

	acc := make([]Message, 0)
	for i := 0; r.Next(); i++ {
		acc = append(acc, Message{})
		err = r.Scan(
			&acc[i].Id,
			&acc[i].Content,
			&acc[i].CreatedAt,
			&acc[i].User.Id,
			&acc[i].User.Email,
			&acc[i].User.FirstName,
			&acc[i].User.LastName,
		)
		if err != nil {
			return err
		}
		acc[i].User.IdNoUnderscore = acc[i].User.Id
	}
	if err = r.Err(); err != nil {
		return err
	}
	*messages = acc
	return nil
}

func (m *manager) SendGlobalMessage(ctx context.Context, projectId sharedTypes.UUID, msg *Message) error {
	if err := checkContent(msg.Content); err != nil {
		return err
	}
	err := m.db.QueryRow(ctx, `
WITH msg AS (
    INSERT INTO chat_messages (id, project_id, content, created_at, user_id)
        VALUES (gen_random_uuid(), $1, $2, transaction_timestamp(), $3)
        RETURNING id, user_id)
SELECT msg.id, users.id, email, first_name, last_name
FROM users, msg
WHERE users.id = msg.user_id
`, projectId, msg.Content, msg.User.Id).Scan(
		&msg.Id,
		&msg.User.Id,
		&msg.User.Email,
		&msg.User.FirstName,
		&msg.User.LastName,
	)
	msg.User.IdNoUnderscore = msg.User.Id
	return err
}

const MaxMessageLength = 10 * 1024

var (
	ErrNoContentProvided = &errors.ValidationError{Msg: "no content provided"}
	ErrContentTooLong    = &errors.ValidationError{
		Msg: "content too long (> 10240 bytes)",
	}
)

func checkContent(content string) error {
	if content == "" {
		return ErrNoContentProvided
	}
	if len(content) > MaxMessageLength {
		return ErrContentTooLong
	}
	return nil
}
