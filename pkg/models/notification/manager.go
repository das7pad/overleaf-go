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

package notification

import (
	"context"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Manager interface {
	GetAllForUser(ctx context.Context, userId sharedTypes.UUID, notifications *[]Notification) error
	Resend(ctx context.Context, notification Notification) error
	RemoveById(ctx context.Context, userId sharedTypes.UUID, notificationId sharedTypes.UUID) error
}

func New(db *pgxpool.Pool) Manager {
	return &manager{db: db}
}

func getErr(_ pgconn.CommandTag, err error) error {
	return err
}

type manager struct {
	db *pgxpool.Pool
}

func (m *manager) GetAllForUser(ctx context.Context, userId sharedTypes.UUID, notifications *[]Notification) error {
	r, err := m.db.Query(ctx, `
SELECT id, key, expires_at, template_key, message_options
FROM notifications
WHERE user_id = $1
  AND template_key != ''
`, userId)
	if err != nil {
		return err
	}
	defer r.Close()

	acc := make([]Notification, 0)
	for i := 0; r.Next(); i++ {
		acc = append(acc, Notification{})
		err = r.Scan(
			&acc[i].Id,
			&acc[i].Key,
			&acc[i].Expires,
			&acc[i].TemplateKey,
			&acc[i].MessageOptions,
		)
		if err != nil {
			return err
		}
	}
	if err = r.Err(); err != nil {
		return err
	}
	*notifications = acc
	return nil
}

func (m *manager) Resend(ctx context.Context, n Notification) error {
	if n.Key == "" {
		return &errors.ValidationError{Msg: "add notification: missing key"}
	}
	return getErr(m.db.Exec(ctx, `
UPDATE notifications
SET expires_at      = $3,
    template_key    = $4,
    message_options = $5
WHERE id = $1
  AND user_id = $2
`, n.Key, n.UserId, n.Expires, n.Key, n.MessageOptions))
}

func (m *manager) RemoveById(ctx context.Context, userId sharedTypes.UUID, notificationId sharedTypes.UUID) error {
	return getErr(m.db.Exec(ctx, `
UPDATE notifications
SET template_key    = '',
    message_options = '{}'
WHERE id = $1
  AND user_id = $2
`, notificationId, userId))
}
