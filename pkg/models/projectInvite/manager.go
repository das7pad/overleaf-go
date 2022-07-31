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

	"github.com/lib/pq"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Manager interface {
	Accept(ctx context.Context, projectId, userId sharedTypes.UUID, token Token) error
	Delete(ctx context.Context, projectId, inviteId sharedTypes.UUID) error
	CheckExists(ctx context.Context, projectId sharedTypes.UUID, token Token) error
	Create(ctx context.Context, pi *WithToken) error
	GetById(ctx context.Context, projectId, inviteId, actorId sharedTypes.UUID) (*WithToken, error)
	GetAllForProject(ctx context.Context, projectId, userId sharedTypes.UUID) ([]ForListing, error)
}

func New(db *sql.DB) Manager {
	return &manager{db: db}
}

func getErr(_ sql.Result, err error) error {
	return err
}

type manager struct {
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
WITH p AS (SELECT id, name
           FROM projects p
           WHERE id = $1
             AND deleted_at IS NULL
             AND owner_id = $2),
     pi AS (
         INSERT INTO project_invites
             (created_at, email, expires_at, id, privilege_level, project_id,
              sending_user_id, token)
             SELECT transaction_timestamp(),
                    $3,
                    $4,
                    gen_random_uuid(),
                    $5,
                    $1,
                    $2,
                    $6
             FROM p
             RETURNING id, project_id, email)
INSERT
INTO notifications
(expires_at, id, key, message_options, template_key, user_id)
SELECT $4,
       gen_random_uuid(),
       concat('project-invite-', pi.id::TEXT),
       jsonb_build_object(
               'userName', $7::TEXT,
               'projectId', $1,
               'projectName', p.name,
               'token', $6
           ),
       'notification_project_invite',
       u.id
FROM users u
         INNER JOIN pi ON u.email = pi.email
         INNER JOIN p ON p.id = pi.project_id
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
			if e, ok := err.(*pq.Error); ok &&
				e.Constraint == "project_invites_project_id_token_key" {
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
	return getErr(m.db.ExecContext(ctx, `
WITH pi AS (
    DELETE FROM project_invites pi USING projects p
        WHERE pi.id = $2
            AND project_id = $1
            AND expires_at > transaction_timestamp()
            AND project_id = p.id
            AND p.deleted_at IS NULL
        RETURNING pi.id, pi.project_id),
     p AS (
         UPDATE projects p
             SET epoch = epoch + 1
             FROM pi
             WHERE p.id = pi.project_id)
DELETE
FROM notifications USING pi
WHERE key = concat('project-invite-', pi.id::TEXT)
`, projectId, inviteId))
}

func (m *manager) GetById(ctx context.Context, projectId, inviteId, actorId sharedTypes.UUID) (*WithToken, error) {
	pi := WithToken{}
	pi.ProjectId = projectId
	return &pi, m.db.QueryRowContext(ctx, `
SELECT created_at, email, expires_at, privilege_level, token
FROM project_invites pi
         INNER JOIN projects p ON p.id = pi.project_id
WHERE pi.id = $2
  AND project_id = $1
  AND expires_at > transaction_timestamp()
  AND p.deleted_at IS NULL
  AND p.owner_id = $3
`, projectId, inviteId, actorId).Scan(
		&pi.CreatedAt,
		&pi.Email,
		&pi.Expires,
		&pi.PrivilegeLevel,
		&pi.Token,
	)
}

func (m *manager) GetAllForProject(ctx context.Context, projectId, userId sharedTypes.UUID) ([]ForListing, error) {
	r, err := m.db.QueryContext(ctx, `
SELECT pi.email, pi.id, pi.privilege_level
FROM project_invites pi
         INNER JOIN projects p ON p.id = pi.project_id
WHERE p.id = $1
  AND p.owner_id = $2
`, projectId, userId)
	if err != nil {
		return nil, err
	}
	defer func() { _ = r.Close() }()
	invites := make([]ForListing, 0)
	for i := 0; r.Next(); i++ {
		invites = append(invites, ForListing{})
		err = r.Scan(
			&invites[i].Email,
			&invites[i].Id,
			&invites[i].PrivilegeLevel,
		)
		if err != nil {
			return nil, err
		}
	}
	if err = r.Err(); err != nil {
		return nil, err
	}
	return invites, nil
}

func (m *manager) Accept(ctx context.Context, projectId, userId sharedTypes.UUID, token Token) error {
	return getErr(m.db.ExecContext(ctx, `
WITH pi AS (
    DELETE FROM project_invites pi USING projects p
        WHERE token = $3
            AND expires_at > transaction_timestamp()
            AND project_id = p.id
            AND p.deleted_at IS NULL
        RETURNING pi.id, project_id, privilege_level, sending_user_id),
     notification as (
         DELETE
             FROM notifications USING pi
                 WHERE key = concat('project-invite-', pi.id::TEXT)),
     contacts AS (
         WITH sortedIds AS (
             WITH ids AS (SELECT $2::UUID AS x, pi.sending_user_id AS y
                          FROM pi)
             SELECT least(ids.x, ids.y) AS a, greatest(ids.x, ids.y) AS b
             FROM ids),
             target AS (
                 WITH prev AS (SELECT connections
                               FROM contacts c,
                                    sortedIds ids
                               WHERE c.a = ids.a
                                 AND c.b = ids.b)
                 SELECT coalesce((SELECT connections FROM prev), 1) AS n)
             INSERT INTO contacts (a, b, connections, last_touched)
                 SELECT sortedIds.a,
                        sortedIds.b,
                        target.n,
                        transaction_timestamp()
                 FROM sortedIds,
                      target
                 ON CONFLICT (a, b) DO UPDATE
                     SET connections = excluded.connections,
                         last_touched = transaction_timestamp()),
     new_entry AS (
         INSERT INTO project_members
             (project_id, user_id, access_source, privilege_level, archived,
              trashed)
             SELECT p.id, $2, 'invite', pi.privilege_level, FALSE, FALSE
             FROM projects p
                      INNER JOIN pi ON p.id = pi.project_id
             WHERE p.id = $1
               AND deleted_at IS NULL
             ON CONFLICT (project_id, user_id) DO NOTHING)
UPDATE project_members pm
SET access_source   = 'invite',
    privilege_level = greatest(pm.privilege_level, pi.privilege_level)
FROM pi
WHERE pm.project_id = pi.project_id
  AND pm.user_id = $2
  AND (pm.access_source = 'token' OR pm.privilege_level < pi.privilege_level)
`, projectId, userId, token))
}

func (m *manager) CheckExists(ctx context.Context, projectId sharedTypes.UUID, token Token) error {
	exists := false
	err := m.db.QueryRowContext(ctx, `
SELECT TRUE
FROM project_invites
WHERE token = $2
  AND project_id = $1
  AND expires_at > transaction_timestamp()
`, projectId, token).Scan(&exists)
	if err == sql.ErrNoRows {
		return &errors.NotFoundError{}
	}
	return err
}
