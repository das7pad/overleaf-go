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
	"database/sql"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Manager interface {
	GetGlobalMessages(
		ctx context.Context,
		projectId sharedTypes.UUID,
		limit int64,
		before sharedTypes.Timestamp,
		target *[]Message,
	) error

	SendGlobalMessage(
		ctx context.Context,
		projectId sharedTypes.UUID,
		msg *Message,
	) error
}

func New(db *sql.DB) Manager {
	return &manager{db: db}
}

type manager struct {
	db *sql.DB
}

func (m *manager) GetGlobalMessages(
	ctx context.Context,
	projectId sharedTypes.UUID,
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
	r, err := m.db.QueryContext(ctx, `
SELECT cm.id,
       cm.content,
       cm.created_at,
       users.id,
       email,
       first_name,
       last_name
FROM users
         INNER JOIN chat_messages cm on users.id = cm.user_id
WHERE cm.project_id = $1
  AND cm.created_at < $2
ORDER BY cm.created_at DESC
LIMIT $3
`, projectId.String(), t, limit)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()

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

func (m *manager) SendGlobalMessage(
	ctx context.Context,
	projectId sharedTypes.UUID,
	msg *Message,
) error {
	if err := checkContent(msg.Content); err != nil {
		return err
	}
	r := m.db.QueryRowContext(ctx, `
WITH msg AS (
    INSERT INTO chat_messages (id, project_id, content, created_at, user_id)
        VALUES (gen_random_uuid(), $1, $2, now(), $3)
        RETURNING id, user_id)
SELECT msg.id, users.id, email, first_name, last_name
FROM users, msg
WHERE users.id = msg.user_id
`, projectId, msg.Content, msg.User.Id.String())
	if err := r.Err(); err != nil {
		return err
	}
	err := r.Scan(
		&msg.Id,
		&msg.User.Id,
		&msg.User.Email,
		&msg.User.FirstName,
		&msg.User.LastName,
	)
	if err != nil {
		return err
	}
	msg.User.IdNoUnderscore = msg.User.Id
	return nil
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
