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

package projectInvite

import (
	"context"

	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type Manager interface {
	Delete(ctx context.Context, projectId, inviteId edgedb.UUID) error
	Create(ctx context.Context, pi *WithToken) error
	GetById(ctx context.Context, projectId, inviteId edgedb.UUID, pi *WithToken) error
	GetByToken(ctx context.Context, projectId edgedb.UUID, token Token, target interface{}) error
	GetAllForProject(ctx context.Context, projectId edgedb.UUID, invites *[]ForListing) error
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

func (m *manager) Create(ctx context.Context, pi *WithToken) error {
	allErrors := &errors.MergedError{}
	for i := 0; i < 10; i++ {
		{
			token, err := generateNewToken()
			if err != nil {
				allErrors.Add(err)
				continue
			}
			pi.Token = token
		}

		var r bool
		err := m.c.QuerySingle(ctx, `
with
	u := (select User filter .email.email = <str>$0 limit 1),
	p := (
		select Project
		filter .id = <uuid>$3 and .owner.id = <uuid>$4
	),
	pi := (insert ProjectInvite {
		email := <str>$0,
		expires_at := <datetime>$1,
		privilege_level := <str>$2,
		project := p,
		sending_user := (select User filter .id = <uuid>$4),
		token := <str>$5,
	})
select exists (
	for entry in ({1} if exists u else <int64>{}) union (
		insert Notification {
			key := 'project-invite-' ++ <str>pi.id,
			expires_at := <datetime>$1,
			user := u,
			template_key := 'notification_project_invite',
			message_options := <json>{
				userName := <str>$6,
				projectName := p.name,
				projectId := <str>p.id,
				token := <str>$5,
			},
		}
	)
)
`,
			&r,
			pi.Email,
			pi.Expires,
			pi.PrivilegeLevel,
			pi.ProjectId,
			pi.SendingUser.Id,
			pi.Token,
			pi.SendingUser.DisplayName(),
		)
		if err != nil {
			if e, ok := err.(edgedb.Error); ok && e.Category(edgedb.ConstraintViolationError) {
				// Duplicate .token
				allErrors.Add(err)
				continue
			}
			return rewriteEdgedbError(err)
		}
		return nil
	}
	return allErrors.Finalize()
}

func (m *manager) Delete(ctx context.Context, projectId, inviteId edgedb.UUID) error {
	var r bool
	err := m.c.QuerySingle(ctx, `
with
	pi := (
		delete ProjectInvite
		filter
			.id = <uuid>$0
		and .project.id = <uuid>$1
		and .expires_at > datetime_of_transaction()
	),
	p := (
		update Project
		filter .id = pi.project.id
		set {
			epoch := Project.epoch + 1
		}
	),
	n := (
		update Notification
		filter .key = 'project-invite-' ++ <str>pi.id
		set {
			template_key := '',
		}
	)
select exists {pi, p, n}
`, &r, inviteId, projectId)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	if !r {
		return &errors.NotFoundError{}
	}
	return nil
}

func (m *manager) GetById(ctx context.Context, projectId, inviteId edgedb.UUID, pi *WithToken) error {
	return rewriteEdgedbError(m.c.QuerySingle(ctx, `
select ProjectInvite {
	created_at,
	email,
	expires_at,
	id,
	privilege_level,
	project,
	sending_user,
	token,
}
filter
	.id = <uuid>$0
and	.project.id = <uuid>$1
and .expires_at > datetime_of_transaction()
limit 1
`, pi, inviteId, projectId))
}

func (m *manager) GetByToken(ctx context.Context, projectId edgedb.UUID, token Token, pi interface{}) error {
	var q string
	switch pi.(type) {
	case *IdField:
		q = `
select ProjectInvite
filter
	.project.id = <uuid>$0
and .token = <str>$1
and .expires_at > datetime_of_transaction()
limit 1
`
	case *WithoutToken:
		q = `
select ProjectInvite {
	created_at,
	email,
	expires_at,
	id,
	privilege_level,
	project,
	sending_user,
}
filter
	.project.id = <uuid>$0
and .token = <str>$1
and .expires_at > datetime_of_transaction()
limit 1
`
	}
	return rewriteEdgedbError(m.c.QuerySingle(ctx, q, pi, projectId, token))
}

func (m *manager) GetAllForProject(ctx context.Context, projectId edgedb.UUID, invites *[]ForListing) error {
	return rewriteEdgedbError(m.c.Query(ctx, `
select (select Project filter .id = <uuid>$0).invites {
	email,
	id,
	privilege_level,
}
`, invites, projectId))
}
