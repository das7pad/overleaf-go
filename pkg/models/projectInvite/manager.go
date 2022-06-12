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

package projectInvite

import (
	"context"
	"database/sql"

	"github.com/edgedb/edgedb-go"
	"github.com/lib/pq"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Manager interface {
	Accept(ctx context.Context, projectId, userId sharedTypes.UUID, token Token) error
	Delete(ctx context.Context, projectId, inviteId sharedTypes.UUID) error
	CheckExists(ctx context.Context, projectId sharedTypes.UUID, token Token) error
	Create(ctx context.Context, pi *WithToken) error
	GetById(ctx context.Context, projectId, inviteId sharedTypes.UUID, pi *WithToken) error
	GetAllForProject(ctx context.Context, projectId, userId sharedTypes.UUID, invites *[]ForListing) error
}

func New(db *sql.DB) Manager {
	return &manager{db: db}
}

func rewriteEdgedbError(err error) error {
	if e, ok := err.(edgedb.Error); ok && e.Category(edgedb.NoDataError) {
		return &errors.NotFoundError{}
	}
	return err
}

func getErr(_ sql.Result, err error) error {
	return err
}

type manager struct {
	c  *edgedb.Client
	db *sql.DB
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

		err := getErr(m.db.ExecContext(ctx, `
WITH pi AS (
    INSERT INTO project_invites
        (created_at, email, expires_at, id, privilege_level, project_id,
         sending_user_id, token)
        SELECT transaction_timestamp(),
               $3,
               $4,
               gen_random_uuid(),
               $5,
               p.id,
               p.owner_id,
               $6
        FROM projects p
        WHERE p.id = $1
          AND p.deleted_at IS NULL
          AND p.owner_id = $2
        RETURNING id, email, p.name AS project_name)
INSERT
INTO notifications
(expires_at, id, key, message_options, template_key, user_id)
SELECT $4,
       gen_random_uuid(),
       concat('project-invite-', pi.id::TEXT),
       jsonb_build_object(
           'userName', $7,
           'projectId', $1,
           'projectName', pi.project_name,
           'token', $6,
       ),
       'notification_project_invite',
       u.id
FROM users u
         INNER JOIN pi ON u.email = pi.email
WHERE u.deleted_at IS NULL
`,
			pi.ProjectId,
			pi.SendingUser.Id,
			pi.Email,
			pi.Expires,
			pi.PrivilegeLevel,
			pi.Token,
			pi.SendingUser.DisplayName(),
		))
		if err != nil {
			if e, ok := err.(*pq.Error); ok && e.Constraint == "project_invites_project_id_token_key" {
				// Duplicate .token
				allErrors.Add(err)
				continue
			}
			return err
		}
		return nil
	}
	return allErrors.Finalize()
}

func (m *manager) Delete(ctx context.Context, projectId, inviteId sharedTypes.UUID) error {
	var r bool
	err := m.c.QuerySingle(ctx, `
with
	pi := (
		delete ProjectInvite
		filter
			.id = <uuid>$0
		and .project.id = <uuid>$1
 		and not exists .project.deleted_at
		and .expires_at > datetime_of_transaction()
	),
	p := (
		update Project
		filter .id = pi.project.id
		set {
			epoch := Project.epoch + 1
		}
	),
select exists {pi, p}
`, &r, inviteId, projectId)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	if !r {
		return &errors.NotFoundError{}
	}
	return nil
}

func (m *manager) GetById(ctx context.Context, projectId, inviteId sharedTypes.UUID, pi *WithToken) error {
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
and not exists .project.deleted_at
and .expires_at > datetime_of_transaction()
limit 1
`, pi, inviteId, projectId))
}

func (m *manager) GetAllForProject(ctx context.Context, projectId, userId sharedTypes.UUID, invites *[]ForListing) error {
	return rewriteEdgedbError(m.c.Query(ctx, `
with
	u := (select User filter .id = <uuid>$1 and not exists .deleted_at),
	p := (select Project filter .id = <uuid>$0 and not exists .deleted_at),
	pWithAuth := (select p filter .owner = u),
select pWithAuth.invites {
	email,
	id,
	privilege_level,
}
`, invites, projectId, userId))
}

func (m *manager) Accept(ctx context.Context, projectId, userId sharedTypes.UUID, token Token) error {
	return getErr(m.db.ExecContext(ctx, `
WITH pi AS (
    DELETE FROM project_invites USING projects p
        WHERE token = $3 AND expires_at > transaction_timestamp()
            AND project_id = p.id AND p.deleted_at IS NULL
        RETURNING project_id, privilege_level, sending_user_id),
     sortedIds AS (WITH ids AS (SELECT $2::UUID as id
                                UNION
                                SELECT pi.sending_user_id as id)
                   SELECT (SELECT id
                           FROM ids
                           ORDER BY ids.id DESC
                           LIMIT 1) AS a,
                          (SELECT id
                           FROM ids
                           ORDER BY ids.id
                           LIMIT 1) AS b),
     invite AS (
         INSERT INTO contacts (a, b, connections, last_touched)
             SELECT (sortedIds.a, sortedIds.b, 1, transaction_timestamp())
             FROM sortedIds
             ON CONFLICT DO UPDATE
                 SET connections = connections + 1,
                     last_touched = transaction_timestamp())
INSERT
INTO project_members
(project_id, user_id, access_source, privilege_level, archived, trashed)
SELECT p.id, $2, 'invite', pi.privilege_level, FALSE, FALSE
FROM projects p
         INNER JOIN pi ON p.id = pi.project_id
WHERE p.id = $1
  AND deleted_at IS NULL

ON CONFLICT (project_id, user_id)
WHERE access_source = 'token'
   OR privilege_level < pi.privilege_level
    DO
UPDATE
SET access_source   = 'invite',
    privilege_level = min(excluded.privilege_level, pi.privilege_level)
`, projectId, userId, token))
}

func (m *manager) CheckExists(ctx context.Context, projectId sharedTypes.UUID, token Token) error {
	exists := false
	err := m.db.QueryRowContext(ctx, `
SELECT TRUE FROM project_invites WHERE token = $2 AND project_id = $1 AND expires_at > transaction_timestamp()
`, projectId, token).Scan(&exists)
	if err == sql.ErrNoRows {
		return &errors.NotFoundError{}
	}
	return err
}
