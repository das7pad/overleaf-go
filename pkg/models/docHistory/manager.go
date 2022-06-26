// Golang port of Overleaf
// Copyright (C) 2022 Jakob Ackermann <das7pad@outlook.com>
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

package docHistory

import (
	"context"
	"database/sql"
	"time"

	"github.com/lib/pq"
	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Manager interface {
	InsertBulk(ctx context.Context, docId sharedTypes.UUID, dh []ForInsert) error
	GetLastVersion(ctx context.Context, projectId, docId sharedTypes.UUID) (sharedTypes.Version, error)
	GetForDoc(ctx context.Context, projectId, userId, docId sharedTypes.UUID, from, to sharedTypes.Version, r *GetForDocResult) error
	GetForProject(ctx context.Context, projectId, userId sharedTypes.UUID, before time.Time, limit int64, r *GetForProjectResult) error
}

func New(db *sql.DB) Manager {
	return &manager{db: db}
}

type manager struct {
	db *sql.DB
}

func (m *manager) InsertBulk(ctx context.Context, docId sharedTypes.UUID, dh []ForInsert) error {
	// NOTE: Leaving out the projectId here relies on a previous call to
	//        GetLastVersion to flag mismatched ids.
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Tag(err, "start tx")
	}
	ok := false
	defer func() {
		if !ok {
			_ = tx.Rollback()
		}
	}()
	q, err := tx.PrepareContext(
		ctx,
		pq.CopyIn(
			"doc_history",
			"id", "doc_id", "user_id", "version", "op",
			"has_big_delete", "start_at", "end_at",
		),
	)
	if err != nil {
		return errors.Tag(err, "prepare bulk insert")
	}
	defer func() {
		if !ok && q != nil {
			_ = q.Close()
		}
	}()

	b, err := sharedTypes.GenerateUUIDBulk(len(dh))
	if err != nil {
		return err
	}
	for i := 0; i < len(dh); i++ {
		for _, component := range dh[i].Op {
			if len(component.Deletion) > 16 {
				dh[i].HasBigDelete = true
				break
			}
		}
		_, err = q.ExecContext(
			ctx,
			b.Next(),
			docId,
			dh[i].UserId,
			dh[i].Version,
			dh[i].Op,
			dh[i].HasBigDelete,
			dh[i].StartAt,
			dh[i].EndAt,
		)
		if err != nil {
			return errors.Tag(err, "queue")
		}
	}
	if _, err = q.ExecContext(ctx); err != nil {
		return errors.Tag(err, "flush bulk insert")
	}
	if err = q.Close(); err != nil {
		return errors.Tag(err, "close bulk insert")
	}

	if err = tx.Commit(); err != nil {
		return errors.Tag(err, "commit tx")
	}
	ok = true
	return nil
}

func (m *manager) GetLastVersion(ctx context.Context, projectId, docId sharedTypes.UUID) (sharedTypes.Version, error) {
	var v sharedTypes.Version
	// NOTE: Going in reverse here allows us to categorize a 404 as actual
	//        missing doc -- and more importantly: mismatched ids.
	err := m.db.QueryRowContext(ctx, `
SELECT coalesce(dh.version, -1)
FROM projects p
INNER JOIN tree_nodes t ON p.id = t.project_id
INNER JOIN docs d ON t.id = d.id
LEFT JOIN doc_history dh ON d.id = dh.doc_id
WHERE p.id = $1 AND t.id = $2
ORDER BY dh.version DESC
LIMIT 1
`, projectId, docId).Scan(&v)
	if err == sql.ErrNoRows {
		return 0, &errors.NotFoundError{}
	}
	return v, err
}

type GetForDocResult struct {
	History []DocHistory
	Users   user.BulkFetched
}

func (m *manager) GetForDoc(ctx context.Context, projectId, userId, docId sharedTypes.UUID, from, to sharedTypes.Version, res *GetForDocResult) error {
	eg, pCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		r, err := m.db.QueryContext(pCtx, `
SELECT dh.version,
       dh.start_at,
       dh.end_at,
       dh.op,
       coalesce(dh.user_id, '00000000-0000-0000-0000-000000000000'::UUID)
FROM doc_history dh
         INNER JOIN docs d ON d.id = dh.doc_id
         INNER JOIN tree_nodes t ON d.id = t.id
         INNER JOIN projects p ON t.project_id = p.id
         INNER JOIN project_members pm ON p.id = pm.project_id
WHERE p.id = $1
  AND pm.user_id = $2
  AND (pm.access_source > 'token' OR
       pm.privilege_level > 'readOnly')
  AND t.id = $3
  AND dh.version >= $4
  AND dh.version <= $5
ORDER BY dh.version
`, projectId, userId, docId, from, to)
		if err != nil {
			return err
		}
		defer func() { _ = r.Close() }()
		h := res.History

		for i := 0; r.Next(); i++ {
			h = append(h, DocHistory{})
			err = r.Scan(
				&h[i].Version,
				&h[i].StartAt,
				&h[i].EndAt,
				&h[i].Op,
				&h[i].UserId,
			)
			if err != nil {
				return err
			}
		}
		res.History = h
		return err
	})
	eg.Go(func() error {
		r, err := m.db.QueryContext(pCtx, `
SELECT DISTINCT ON (u.id) u.id, u.email, u.first_name, u.last_name
FROM doc_history dh
         INNER JOIN docs d ON d.id = dh.doc_id
         INNER JOIN tree_nodes t ON d.id = t.id
         INNER JOIN projects p ON t.project_id = p.id
         INNER JOIN users u ON dh.user_id = u.id
         INNER JOIN project_members pm ON p.id = pm.project_id
WHERE p.id = $1
  AND pm.user_id = $2
  AND (pm.access_source > 'token' OR
       pm.privilege_level > 'readOnly')
  AND t.id = $3
  AND dh.version >= $4
  AND dh.version <= $5
  AND u.deleted_at IS NULL
`, projectId, userId, docId, from, to)
		if err != nil {
			return err
		}
		defer func() { _ = r.Close() }()
		c := res.Users

		for i := 0; r.Next(); i++ {
			c = append(c, user.WithPublicInfo{})
			err = r.Scan(
				&c[i].Id,
				&c[i].Email,
				&c[i].FirstName,
				&c[i].LastName,
			)
			if err != nil {
				return err
			}
		}
		res.Users = c
		return nil
	})
	return eg.Wait()
}

type GetForProjectResult struct {
	History []ProjectUpdate
	Users   user.BulkFetched
}

func (m *manager) GetForProject(ctx context.Context, projectId, userId sharedTypes.UUID, before time.Time, limit int64, res *GetForProjectResult) error {
	eg, pCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		r, err := m.db.QueryContext(pCtx, `
WITH dh AS (SELECT dh.version,
                   dh.start_at,
                   dh.end_at,
                   dh.has_big_delete,
                   dh.user_id,
                   dh.doc_id
            FROM doc_history dh
                     INNER JOIN docs d ON d.id = dh.doc_id
                     INNER JOIN tree_nodes t ON d.id = t.id
                     INNER JOIN projects p ON t.project_id = p.id
                     INNER JOIN project_members pm ON p.id = pm.project_id
            WHERE p.id = $1
              AND pm.user_id = $2
              AND (pm.access_source > 'token' OR
                   pm.privilege_level > 'readOnly')),
     dh_end AS (WITH dh_window AS (SELECT end_at
                                   FROM dh
                                   WHERE end_at < $3
                                   ORDER BY end_at DESC
                                   LIMIT $4)
                SELECT min(end_at) AS end_at_min
                FROM dh_window)

SELECT dh.version,
       dh.start_at,
       dh.end_at,
       dh.has_big_delete,
       coalesce(dh.user_id, '00000000-0000-0000-0000-000000000000'::UUID),
       dh.doc_id
FROM dh,
     dh_end
WHERE dh.end_at < $3
  AND dh.end_at >= dh_end.end_at_min
ORDER BY dh.end_at DESC
`, projectId, userId, before, limit)
		if err != nil {
			return err
		}
		defer func() { _ = r.Close() }()
		h := res.History

		for i := 0; r.Next(); i++ {
			h = append(h, ProjectUpdate{})
			err = r.Scan(
				&h[i].Version,
				&h[i].StartAt,
				&h[i].EndAt,
				&h[i].HasBigDelete,
				&h[i].UserId,
				&h[i].DocId,
			)
			if err != nil {
				return err
			}
		}
		res.History = h
		return err
	})
	eg.Go(func() error {
		r, err := m.db.QueryContext(pCtx, `
WITH dh AS (SELECT dh.*
            FROM doc_history dh
                     INNER JOIN docs d ON d.id = dh.doc_id
                     INNER JOIN tree_nodes t ON d.id = t.id
                     INNER JOIN projects p ON t.project_id = p.id
                     INNER JOIN project_members pm on p.id = pm.project_id
            WHERE p.id = $1
              AND pm.user_id = $2
              AND (pm.access_source > 'token' OR
                   pm.privilege_level > 'readOnly')),
     dh_end AS (WITH dh_window AS (SELECT end_at
                                   FROM dh
                                   WHERE end_at < $3
                                   ORDER BY end_at DESC
                                   LIMIT $4)
                SELECT min(end_at) AS end_at_min
                FROM dh_window)

SELECT DISTINCT ON (u.id) u.id, u.email, u.first_name, u.last_name
FROM dh
         INNER JOIN users u ON dh.user_id = u.id
         INNER JOIN dh_end ON TRUE
WHERE dh.end_at < $3
  AND dh.end_at >= dh_end.end_at_min
  AND u.deleted_at IS NULL
`, projectId, userId, before, limit)
		if err != nil {
			return err
		}
		defer func() { _ = r.Close() }()
		c := res.Users

		for i := 0; r.Next(); i++ {
			c = append(c, user.WithPublicInfo{})
			err = r.Scan(
				&c[i].Id,
				&c[i].Email,
				&c[i].FirstName,
				&c[i].LastName,
			)
			if err != nil {
				return err
			}
		}
		res.Users = c
		return nil
	})
	return eg.Wait()
}
