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

package notification

import (
	"context"

	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type Manager interface {
	GetAllForUser(ctx context.Context, userId edgedb.UUID, notifications *[]Notification) error
	Resend(ctx context.Context, notification Notification) error
	RemoveById(ctx context.Context, userId edgedb.UUID, notificationId edgedb.UUID) error
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

func (m *manager) GetAllForUser(ctx context.Context, userId edgedb.UUID, notifications *[]Notification) error {
	err := m.c.Query(
		ctx,
		`
select Notification {
	key,
	expires_at,
	template_key,
	message_options,
}
filter .user.id = <uuid>$0 and .template_key != ''`,
		notifications,
		userId,
	)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	return nil
}

func (m *manager) Resend(ctx context.Context, n Notification) error {
	if n.Key == "" {
		return &errors.ValidationError{
			Msg: "cannot add notification: missing key",
		}
	}
	err := m.c.QuerySingle(
		ctx,
		`
update Notification
filter .key = <str>$1 and .user.id = <uuid>$0
set {
	expires_at := <datetime>$2,
	template_key := <str>$3,
	message_options := <json>$4,
}
)`,
		&IdField{},
		n.UserId, n.Key, n.Expires, n.TemplateKey, []byte(n.MessageOptions),
	)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	return nil
}

func (m *manager) RemoveById(ctx context.Context, userId edgedb.UUID, notificationId edgedb.UUID) error {
	err := m.c.QuerySingle(
		ctx,
		`
update Notification
filter .id = <uuid>$0 and .user.id = <uuid>$1
set { template_key := {}, message_options := {} }
`,
		&IdField{},
		notificationId, userId,
	)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	return nil
}
