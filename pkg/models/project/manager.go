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

package project

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/notification"
	"github.com/das7pad/overleaf-go/pkg/models/tag"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	spellingTypes "github.com/das7pad/overleaf-go/services/spelling/pkg/types"
)

type Manager interface {
	PrepareProjectCreation(ctx context.Context, p *ForCreation) error
	FinalizeProjectCreation(ctx context.Context, p *ForCreation) error
	SoftDelete(ctx context.Context, projectIds sharedTypes.UUIDs, userId sharedTypes.UUID, ipAddress string) error
	HardDelete(ctx context.Context, projectId sharedTypes.UUID) error
	ProcessSoftDeleted(ctx context.Context, cutOff time.Time, fn func(projectId sharedTypes.UUID) bool) error
	GetDeletedProjectsName(ctx context.Context, projectId, userId sharedTypes.UUID) (Name, error)
	Restore(ctx context.Context, projectId, userId sharedTypes.UUID, name Name) error
	AddFolder(ctx context.Context, projectId, userId, parent sharedTypes.UUID, f *Folder) (sharedTypes.Version, error)
	DeleteDoc(ctx context.Context, projectId, userId, docId sharedTypes.UUID) (sharedTypes.Version, error)
	DeleteFile(ctx context.Context, projectId, userId, fileId sharedTypes.UUID) (sharedTypes.Version, error)
	DeleteFolder(ctx context.Context, projectId, userId, folderId sharedTypes.UUID) (sharedTypes.Version, error)
	RestoreDoc(ctx context.Context, projectId, userId, docId sharedTypes.UUID, name sharedTypes.Filename) (sharedTypes.Version, sharedTypes.UUID, error)
	MoveDoc(ctx context.Context, projectId, userId, folderId, docId sharedTypes.UUID) (sharedTypes.Version, sharedTypes.PathName, error)
	MoveFile(ctx context.Context, projectId, userId, folderId, fileId sharedTypes.UUID) (sharedTypes.Version, sharedTypes.PathName, error)
	MoveFolder(ctx context.Context, projectId, userId, targetFolderId, folderId sharedTypes.UUID) (sharedTypes.Version, []Doc, []FileRef, error)
	RenameDoc(ctx context.Context, projectId, userId sharedTypes.UUID, d *Doc) (sharedTypes.Version, sharedTypes.PathName, error)
	RenameFile(ctx context.Context, projectId, userId sharedTypes.UUID, f *FileRef) (sharedTypes.Version, sharedTypes.PathName, error)
	RenameFolder(ctx context.Context, projectId, userId sharedTypes.UUID, f *Folder) (sharedTypes.Version, []Doc, []FileRef, error)
	GetAuthorizationDetails(ctx context.Context, projectId, userId sharedTypes.UUID, accessToken AccessToken) (*AuthorizationDetails, error)
	GetForClone(ctx context.Context, projectId, userId sharedTypes.UUID) (*ForClone, error)
	GetForProjectInvite(ctx context.Context, projectId, actorId sharedTypes.UUID, email sharedTypes.Email) (*ForProjectInvite, error)
	GetForProjectJWT(ctx context.Context, projectId, userId sharedTypes.UUID, accessToken AccessToken) (*ForProjectJWT, int64, error)
	GetForZip(ctx context.Context, projectId sharedTypes.UUID, userId sharedTypes.UUID, accessToken AccessToken) (*ForZip, error)
	ValidateProjectJWTEpochs(ctx context.Context, projectId, userId sharedTypes.UUID, projectEpoch, userEpoch int64) error
	BumpLastOpened(ctx context.Context, projectId sharedTypes.UUID) error
	GetDoc(ctx context.Context, projectId, docId sharedTypes.UUID) (*Doc, error)
	GetFile(ctx context.Context, projectId, userId sharedTypes.UUID, accessToken AccessToken, fileId sharedTypes.UUID) (*FileWithParent, error)
	GetElementByPath(ctx context.Context, projectId, userId sharedTypes.UUID, path sharedTypes.PathName) (sharedTypes.UUID, bool, error)
	GetBootstrapWSDetails(ctx context.Context, projectId, userId sharedTypes.UUID, projectEpoch, userEpoch int64, source AccessSource) (*GetBootstrapWSDetails, error)
	GetLastUpdatedAt(ctx context.Context, projectId sharedTypes.UUID) (time.Time, error)
	GetLoadEditorDetails(ctx context.Context, projectId, userId sharedTypes.UUID, accessToken AccessToken) (*LoadEditorDetails, error)
	GetProjectWithContent(ctx context.Context, projectId sharedTypes.UUID) ([]Doc, []FileRef, error)
	GetTokenAccessDetails(ctx context.Context, userId sharedTypes.UUID, privilegeLevel sharedTypes.PrivilegeLevel, accessToken AccessToken) (*ForTokenAccessDetails, *AuthorizationDetails, error)
	GetTreeEntities(ctx context.Context, projectId, userId sharedTypes.UUID) ([]TreeEntity, error)
	GetProjectMembers(ctx context.Context, projectId sharedTypes.UUID) ([]user.AsProjectMember, error)
	GrantTokenAccess(ctx context.Context, projectId, userId sharedTypes.UUID, accessToken AccessToken, privilegeLevel sharedTypes.PrivilegeLevel) error
	GrantMemberAccess(ctx context.Context, projectId, ownerId, userId sharedTypes.UUID, privilegeLevel sharedTypes.PrivilegeLevel) error
	GetAccessTokens(ctx context.Context, projectId, userId sharedTypes.UUID, tokens *Tokens) error
	PopulateTokens(ctx context.Context, projectId, userId sharedTypes.UUID) (*Tokens, error)
	GetProjectNames(ctx context.Context, userId sharedTypes.UUID) (Names, error)
	SetCompiler(ctx context.Context, projectId, userId sharedTypes.UUID, compiler sharedTypes.Compiler) error
	SetImageName(ctx context.Context, projectId, userId sharedTypes.UUID, imageName sharedTypes.ImageName) error
	SetSpellCheckLanguage(ctx context.Context, projectId, userId sharedTypes.UUID, spellCheckLanguage spellingTypes.SpellCheckLanguage) error
	SetRootDoc(ctx context.Context, projectId, userId, rooDocId sharedTypes.UUID) error
	SetPublicAccessLevel(ctx context.Context, projectId, userId sharedTypes.UUID, level PublicAccessLevel) error
	ArchiveForUser(ctx context.Context, projectId, userId sharedTypes.UUID) error
	UnArchiveForUser(ctx context.Context, projectId, userId sharedTypes.UUID) error
	TrashForUser(ctx context.Context, projectId, userId sharedTypes.UUID) error
	UnTrashForUser(ctx context.Context, projectId, userId sharedTypes.UUID) error
	Rename(ctx context.Context, projectId, userId sharedTypes.UUID, name Name) error
	RemoveMember(ctx context.Context, projectId sharedTypes.UUID, actor, userId sharedTypes.UUID) error
	TransferOwnership(ctx context.Context, projectId, previousOwnerId, newOwnerId sharedTypes.UUID) (*user.WithPublicInfo, *user.WithPublicInfo, Name, error)
	CreateDoc(ctx context.Context, projectId, userId, folderId sharedTypes.UUID, d *Doc) (sharedTypes.Version, error)
	EnsureIsDoc(ctx context.Context, projectId, userId, folderId sharedTypes.UUID, d *Doc) (sharedTypes.UUID, bool, sharedTypes.Version, error)
	PrepareFileCreation(ctx context.Context, projectId, userId, folderId sharedTypes.UUID, f *FileRef) error
	FinalizeFileCreation(ctx context.Context, projectId, userId sharedTypes.UUID, f *FileRef) (sharedTypes.UUID, bool, sharedTypes.Version, error)
	ProcessStaleFileUploads(ctx context.Context, cutOff time.Time, fn func(projectId, fileId sharedTypes.UUID) bool) error
	PurgeStaleFileUpload(ctx context.Context, projectId, fileId sharedTypes.UUID) error
	ListProjectsWithName(ctx context.Context, userId sharedTypes.UUID) ([]WithIdAndName, error)
	GetOwnedProjects(ctx context.Context, userId sharedTypes.UUID) ([]sharedTypes.UUID, error)
	GetProjectListDetails(ctx context.Context, userId sharedTypes.UUID, r *ForProjectList) error
}

func New(db *pgxpool.Pool) Manager {
	return &manager{db: db}
}

func getErr(_ pgconn.CommandTag, err error) error {
	return rewritePostgresErr(err)
}

func rewritePostgresErr(err error) error {
	if err == nil {
		return nil
	}
	e, ok := err.(*pgconn.PgError)
	if !ok {
		return err
	}
	if e.ConstraintName == "tree_nodes_project_id_deleted_at_path_key" {
		return ErrDuplicateNameInFolder
	}
	return err
}

type queryRunner interface {
	QueryRow(ctx context.Context, query string, args ...any) pgx.Row
}

type manager struct {
	db *pgxpool.Pool
}

func (m *manager) PrepareProjectCreation(ctx context.Context, p *ForCreation) error {
	ok := false
	tx, err := m.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if !ok {
			_ = tx.Rollback(ctx)
		}
	}()

	_, err = tx.Exec(
		ctx,
		`
WITH p AS (
    INSERT INTO projects
        (created_at, compiler, deleted_at, epoch, id, image_name,
         last_opened_at, last_updated_at, last_updated_by, name, owner_id,
         public_access_level, spell_check_language, tree_version)
        SELECT $7,
               $3,
               $4,
               1,
               $5,
               $6,
               $7,
               $7,
               o.id,
               '',
               o.id,
               'private',
               coalesce(
                       nullif($2, 'inherit'),
                       (o.editor_config ->> 'spellCheckLanguage')
                   ),
               1
        FROM users o
        WHERE o.id = $1
          AND o.deleted_at IS NULL
        RETURNING id, owner_id)
INSERT
INTO project_members
(project_id, user_id, access_source, privilege_level, archived, trashed)
SELECT p.id, p.owner_id, 'owner', 'owner', FALSE, FALSE
FROM p
`,
		p.OwnerId, p.SpellCheckLanguage, p.Compiler, p.DeletedAt, p.Id,
		p.ImageName, p.CreatedAt,
	)
	if err != nil {
		return err
	}

	t := p.RootFolder
	deletedAt := time.Unix(0, 0)
	rows := make([][]interface{}, 0, t.CountNodes())

	rows = append(rows, []interface{}{
		p.CreatedAt, deletedAt, t.Id, TreeNodeKindFolder, nil, "", p.Id,
	})
	_ = t.WalkFolders(func(f *Folder) error {
		for _, d := range f.Docs {
			rows = append(rows, []interface{}{
				p.CreatedAt, deletedAt, d.Id, TreeNodeKindDoc, f.Id,
				f.Path.Join(d.Name), p.Id,
			})
		}
		for _, r := range f.FileRefs {
			rows = append(rows, []interface{}{
				r.CreatedAt, deletedAt, r.Id, TreeNodeKindFile, f.Id,
				f.Path.Join(r.Name), p.Id,
			})
		}
		for _, ff := range f.Folders {
			rows = append(rows, []interface{}{
				p.CreatedAt, deletedAt, ff.Id, TreeNodeKindFolder, f.Id,
				f.Path.Join(ff.Name) + "/", p.Id,
			})
		}
		return nil
	})
	_, err = tx.CopyFrom(
		ctx,
		pgx.Identifier{"tree_nodes"},
		[]string{
			"created_at", "deleted_at", "id", "kind", "parent_id", "path",
			"project_id",
		},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return errors.Tag(err, "insert tree")
	}

	rows = rows[:0]
	_ = t.WalkFolders(func(f *Folder) error {
		for _, d := range f.Docs {
			rows = append(rows, []interface{}{d.Id, d.Snapshot, d.Version})
		}
		return nil
	})
	_, err = tx.CopyFrom(
		ctx,
		pgx.Identifier{"docs"},
		[]string{"id", "snapshot", "version"},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return errors.Tag(err, "insert docs")
	}

	rows = rows[:0]
	_ = t.WalkFolders(func(f *Folder) error {
		for i, ff := range f.FileRefs {
			rows = append(rows, []interface{}{
				ff.Id, ff.Hash, f.FileRefs[i].LinkedFileData, ff.Size, false,
			})
		}
		return nil
	})
	_, err = tx.CopyFrom(
		ctx,
		pgx.Identifier{"files"},
		[]string{"id", "hash", "linked_file_data", "size", "pending"},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return errors.Tag(err, "insert files")
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	ok = true
	return nil
}

func (m *manager) FinalizeProjectCreation(ctx context.Context, p *ForCreation) error {
	var rootDocId interface{} = nil
	if !p.RootDoc.Id.IsZero() {
		rootDocId = p.RootDoc.Id
	}
	return getErr(m.db.Exec(ctx, `
UPDATE projects
SET deleted_at     = NULL,
    name           = $2,
    root_doc_id    = $3,
    root_folder_id = $4
WHERE id = $1
`, p.Id, p.Name, rootDocId, p.RootFolder.Id))
}

func (m *manager) GetAccessTokens(ctx context.Context, projectId, userId sharedTypes.UUID, tokens *Tokens) error {
	return m.db.QueryRow(ctx, `
SELECT coalesce(token_ro, ''), coalesce(token_rw, '')
FROM projects
WHERE id = $1
  AND owner_id = $2
`, projectId, userId).Scan(&tokens.ReadOnly, &tokens.ReadAndWrite)
}

func (m *manager) PopulateTokens(ctx context.Context, projectId, userId sharedTypes.UUID) (*Tokens, error) {
	allErrors := &errors.MergedError{}
	for i := 0; i < 10; i++ {
		tokens, err := generateTokens()
		if err != nil {
			allErrors.Add(err)
			continue
		}
		persisted := Tokens{}
		err = m.db.QueryRow(ctx, `
UPDATE projects
SET token_ro        = coalesce(token_ro, $3),
    token_rw        = coalesce(token_rw, $4),
    token_rw_prefix = coalesce(token_rw_prefix, $5)
WHERE id = $1
  AND owner_id = $2
  AND deleted_at IS NULL
RETURNING token_ro, token_rw
`,
			projectId, userId,
			tokens.ReadOnly, tokens.ReadAndWrite, tokens.ReadAndWritePrefix,
		).Scan(&persisted.ReadOnly, &persisted.ReadAndWrite)
		if err != nil {
			if e, ok := err.(*pgconn.PgError); ok &&
				(e.ConstraintName == "projects_token_ro_key" ||
					e.ConstraintName == "projects_token_rw_prefix_key") {
				allErrors.Add(err)
				continue
			}
			return nil, err
		}
		return &persisted, nil
	}
	return nil, errors.Tag(allErrors, "bad random source")
}

func (m *manager) SetCompiler(ctx context.Context, projectId, userId sharedTypes.UUID, compiler sharedTypes.Compiler) error {
	return getErr(m.db.Exec(ctx, `
UPDATE projects p
SET compiler = $3
FROM project_members pm
WHERE p.id = $1
  AND p.deleted_at IS NULL
  AND p.id = pm.project_id
  AND pm.user_id = $2
  AND pm.privilege_level >= 'readAndWrite'
`, projectId, userId, compiler))
}

func (m *manager) SetImageName(ctx context.Context, projectId, userId sharedTypes.UUID, imageName sharedTypes.ImageName) error {
	return getErr(m.db.Exec(ctx, `
UPDATE projects p
SET image_name = $3
FROM project_members pm
WHERE p.id = $1
  AND p.deleted_at IS NULL
  AND p.id = pm.project_id
  AND pm.user_id = $2
  AND pm.privilege_level >= 'readAndWrite'
`, projectId, userId, imageName))
}

func (m *manager) SetSpellCheckLanguage(ctx context.Context, projectId, userId sharedTypes.UUID, spellCheckLanguage spellingTypes.SpellCheckLanguage) error {
	return getErr(m.db.Exec(ctx, `
UPDATE projects p
SET spell_check_language = $3
FROM project_members pm
WHERE p.id = $1
  AND p.deleted_at IS NULL
  AND p.id = pm.project_id
  AND pm.user_id = $2
  AND pm.privilege_level >= 'readAndWrite'
`, projectId, userId, spellCheckLanguage))
}

func (m *manager) SetRootDoc(ctx context.Context, projectId, userId, rootDocId sharedTypes.UUID) error {
	return getErr(m.db.Exec(ctx, `
WITH d AS (SELECT d.id
           FROM docs d
                    INNER JOIN tree_nodes t ON d.id = t.id
           WHERE d.id = $3
             AND t.project_id = $1
             AND t.deleted_at = '1970-01-01'
             AND (t.path LIKE '%.tex' OR
                  t.path LIKE '%.rtex' OR
                  t.path LIKE '%.ltex'))
UPDATE projects p
SET root_doc_id = d.id
FROM project_members pm,
     d
WHERE p.id = $1
  AND p.deleted_at IS NULL
  AND p.id = pm.project_id
  AND pm.user_id = $2
  AND pm.privilege_level >= 'readAndWrite'
`, projectId, userId, rootDocId))
}

func (m *manager) SetPublicAccessLevel(ctx context.Context, projectId, userId sharedTypes.UUID, publicAccessLevel PublicAccessLevel) error {
	return getErr(m.db.Exec(ctx, `
UPDATE projects
SET public_access_level = $3
WHERE id = $1
  AND owner_id = $2
  AND deleted_at IS NULL
  AND token_ro IS NOT NULL
`, projectId, userId, publicAccessLevel))
}

func (m *manager) TransferOwnership(ctx context.Context, projectId, previousOwnerId, newOwnerId sharedTypes.UUID) (*user.WithPublicInfo, *user.WithPublicInfo, Name, error) {
	previousOwner := user.WithPublicInfo{}
	previousOwner.Id = previousOwnerId
	newOwner := user.WithPublicInfo{}
	newOwner.Id = newOwnerId
	var name Name
	return &previousOwner, &newOwner, name, m.db.QueryRow(ctx, `
WITH ctx AS (SELECT p.id     AS project_id,
                    p.name   AS project_name,
                    o_old.id AS old_owner_id,
                    o_new.id AS new_owner_id
             FROM projects p
                      INNER JOIN project_members pm_old
                                 ON p.id = pm_old.project_id
                      INNER JOIN users o_old ON pm_old.user_id = o_old.id
                      INNER JOIN project_members pm_new
                                 ON p.id = pm_new.project_id
                      INNER JOIN users o_new ON pm_new.user_id = o_new.id
             WHERE p.id = $1
               AND p.owner_id = $2
               AND o_old.id = $2
               AND o_old.deleted_at IS NULL
               AND o_new.id = $3
               AND o_new.deleted_at IS NULL),
     swap_member_old AS (
         UPDATE project_members pm
             SET access_source = 'invite',
                 privilege_level = 'readAndWrite'
             FROM ctx
             WHERE pm.project_id = ctx.project_id AND
                   pm.user_id = ctx.old_owner_id
             RETURNING TRUE),
     swap_member_new AS (
         UPDATE project_members pm
             SET access_source = 'owner',
                 privilege_level = 'owner'
             FROM ctx
             WHERE pm.project_id = ctx.project_id AND
                   pm.user_id = ctx.new_owner_id
             RETURNING TRUE),
     swap_owner AS (
         UPDATE projects p
             SET owner_id = ctx.new_owner_id,
                 epoch = p.epoch + 1
             FROM ctx
             WHERE p.id = ctx.project_id
             RETURNING TRUE),
     log AS (
         INSERT
             INTO project_audit_log
                 (created_at, id, info, initiator_id, operation, project_id)
                 SELECT transaction_timestamp(),
						gen_random_uuid(),
                        json_build_object(
                                'newOwnerId', ctx.new_owner_id,
                                'previousOwnerId', ctx.old_owner_id
                            ),
                        ctx.old_owner_id,
                        'transfer-ownership',
                        ctx.project_id
                 FROM ctx
                 RETURNING TRUE)
SELECT ctx.project_name,
       o_old.email,
       o_old.first_name,
       o_old.last_name,
       o_new.email,
       o_new.first_name,
       o_new.last_name
FROM swap_member_old,
     swap_member_new,
     swap_owner,
     log,
     ctx
         INNER JOIN users o_old ON o_old.id = ctx.old_owner_id
         INNER JOIN users o_new ON o_new.id = ctx.new_owner_id
`, projectId, previousOwnerId, newOwnerId).Scan(
		&name,
		&previousOwner.Email,
		&previousOwner.FirstName,
		&previousOwner.LastName,
		&newOwner.Email,
		&newOwner.FirstName,
		&newOwner.LastName,
	)
}

func (m *manager) Rename(ctx context.Context, projectId, userId sharedTypes.UUID, name Name) error {
	return getErr(m.db.Exec(ctx, `
UPDATE projects
SET name = $3
WHERE id = $1
  AND deleted_at IS NULL
  AND owner_id = $2
`, projectId, userId, name))
}

func (m *manager) AddFolder(ctx context.Context, projectId, userId, parent sharedTypes.UUID, f *Folder) (sharedTypes.Version, error) {
	var treeVersion sharedTypes.Version
	return treeVersion, m.db.QueryRow(ctx, `
WITH f AS (
    INSERT INTO tree_nodes
        (created_at, deleted_at, id, kind, parent_id, path, project_id)
        SELECT transaction_timestamp(),
               '1970-01-01',
               $4,
               'folder',
               $3,
               concat(t.path, $5::TEXT, '/'),
               $1
        FROM projects p
                 INNER JOIN project_members pm ON (p.id = pm.project_id AND
                                                   pm.user_id = $2)
                 INNER JOIN tree_nodes t ON (p.id = t.project_id AND
                                             t.id = $3 AND
                                             t.deleted_at = '1970-01-01')
        WHERE p.id = $1
          AND p.deleted_at IS NULL
          AND pm.privilege_level >= 'readAndWrite'
        RETURNING project_id)
UPDATE projects p
SET tree_version    = tree_version + 1,
    last_updated_at = transaction_timestamp(),
    last_updated_by = $2
FROM f
WHERE p.id = f.project_id
RETURNING p.tree_version
`, projectId, userId, parent, f.Id, f.Name).Scan(&treeVersion)
}

func (m *manager) deleteTreeLeaf(ctx context.Context, projectId, userId, nodeId sharedTypes.UUID, kind TreeNodeKind, runner queryRunner) (sharedTypes.Version, error) {
	var treeVersion sharedTypes.Version
	return treeVersion, runner.QueryRow(ctx, `
WITH node AS (SELECT t.id
              FROM tree_nodes t
                       INNER JOIN projects p ON t.project_id = p.id
                       INNER JOIN project_members pm
                                  ON (t.project_id = pm.project_id AND
                                      pm.user_id = $2)
              WHERE t.id = $3
                AND t.kind = $4
                AND t.project_id = $1
                AND p.deleted_at IS NULL
                AND t.deleted_at = '1970-01-01'
                AND pm.privilege_level >= 'readAndWrite'),
     deleted AS (
         UPDATE tree_nodes t
             SET deleted_at = transaction_timestamp()
             FROM node
             WHERE t.id = node.id
             RETURNING t.id)

UPDATE projects p
SET last_updated_by = $2,
    last_updated_at = transaction_timestamp(),
    root_doc_id     = NULLIF(p.root_doc_id, deleted.id),
    tree_version    = tree_version + 1
FROM deleted
WHERE p.id = $1
RETURNING p.tree_version
`, projectId, userId, nodeId, kind).Scan(&treeVersion)
}

func (m *manager) DeleteDoc(ctx context.Context, projectId, userId, docId sharedTypes.UUID) (sharedTypes.Version, error) {
	return m.deleteTreeLeaf(ctx, projectId, userId, docId, TreeNodeKindDoc, m.db)
}

func (m *manager) DeleteFile(ctx context.Context, projectId, userId, fileId sharedTypes.UUID) (sharedTypes.Version, error) {
	return m.deleteTreeLeaf(ctx, projectId, userId, fileId, TreeNodeKindFile, m.db)
}

func (m *manager) DeleteFolder(ctx context.Context, projectId, userId, folderId sharedTypes.UUID) (sharedTypes.Version, error) {
	var v sharedTypes.Version
	return v, rewritePostgresErr(m.db.QueryRow(ctx, `
WITH node AS (SELECT t.id,
                     t.project_id,
                     t.path
              FROM tree_nodes t
                       INNER JOIN projects p ON t.project_id = p.id
                       INNER JOIN project_members pm
                                  ON (t.project_id = pm.project_id AND
                                      pm.user_id = $2)
                       INNER JOIN tree_nodes parent ON t.parent_id = parent.id
              WHERE t.id = $3
                AND t.kind = 'folder'
                AND t.project_id = $1
                AND t.parent_id IS NOT NULL
                AND p.deleted_at IS NULL
                AND t.deleted_at = '1970-01-01'
                AND pm.privilege_level >= 'readAndWrite'),
     updated_children AS (
         UPDATE tree_nodes t
             SET deleted_at = transaction_timestamp()
             FROM node
             WHERE t.project_id = node.project_id
                 AND t.deleted_at = '1970-01-01'
                 AND starts_with(t.path, node.path)
             RETURNING t.id, t.project_id),
     updated_root_doc AS (SELECT (SELECT c.id
                                  FROM updated_children c
                                           INNER JOIN projects p
                                                      ON c.project_id = p.id
                                  WHERE p.root_doc_id = c.id) AS id)
UPDATE projects p
SET last_updated_by = $2,
    last_updated_at = transaction_timestamp(),
    root_doc_id     = NULLIF(root_doc_id, updated_root_doc.id),
    tree_version    = tree_version + 1
FROM updated_root_doc,
     node
WHERE p.id = node.project_id
RETURNING p.tree_version
`, projectId, userId, folderId).Scan(&v))
}

func (m *manager) moveTreeLeaf(ctx context.Context, projectId, userId, folderId, nodeId sharedTypes.UUID, kind TreeNodeKind) (sharedTypes.Version, sharedTypes.PathName, error) {
	var treeVersion sharedTypes.Version
	var path sharedTypes.PathName
	return treeVersion, path, m.db.QueryRow(ctx, `
WITH f AS (SELECT t.id, t.path, t.project_id
           FROM tree_nodes t
                    INNER JOIN projects p ON t.project_id = p.id
                    INNER JOIN project_members pm
                               ON (t.project_id = pm.project_id AND
                                   pm.user_id = $2)
           WHERE t.id = $3
             AND t.project_id = $1
             AND p.deleted_at IS NULL
             AND t.deleted_at = '1970-01-01'
             AND pm.privilege_level >= 'readAndWrite'),
     updated AS (
         UPDATE tree_nodes t
             SET parent_id = f.id,
                 path = concat(f.path, split_part(t.path, '/', -1))
             FROM f
             WHERE t.id = $4
                 AND t.project_id = f.project_id
                 AND kind = $5
                 AND t.deleted_at = '1970-01-01'
             RETURNING t.id, t.path)

UPDATE projects p
SET last_updated_by = $2,
    last_updated_at = transaction_timestamp(),
    tree_version    = tree_version + 1
FROM updated
WHERE p.id = $1
RETURNING p.tree_version, updated.path
`, projectId, userId, folderId, nodeId, kind).Scan(&treeVersion, &path)
}

func (m *manager) MoveDoc(ctx context.Context, projectId, userId, folderId, docId sharedTypes.UUID) (sharedTypes.Version, sharedTypes.PathName, error) {
	return m.moveTreeLeaf(ctx, projectId, userId, folderId, docId, TreeNodeKindDoc)
}

func (m *manager) MoveFile(ctx context.Context, projectId, userId, folderId, fileId sharedTypes.UUID) (sharedTypes.Version, sharedTypes.PathName, error) {
	return m.moveTreeLeaf(ctx, projectId, userId, folderId, fileId, TreeNodeKindFile)
}

func (m *manager) MoveFolder(ctx context.Context, projectId, userId, targetFolderId, folderId sharedTypes.UUID) (sharedTypes.Version, []Doc, []FileRef, error) {
	var v sharedTypes.Version
	var docIds sharedTypes.UUIDs
	var docPaths []string
	var fileIds sharedTypes.UUIDs
	var filePaths []string
	err := m.db.QueryRow(ctx, `
WITH node AS (SELECT t.id,
                     t.project_id,
                     t.path,
                     char_length(t.path) + 1     AS old_end,
                     split_part(t.path, '/', -2) AS name
              FROM tree_nodes t
                       INNER JOIN projects p ON t.project_id = p.id
                       INNER JOIN project_members pm
                                  ON (t.project_id = pm.project_id AND
                                      pm.user_id = $2)
              WHERE t.id = $3
                AND t.kind = 'folder'
                AND t.project_id = $1
                AND t.parent_id IS NOT NULL
                AND p.deleted_at IS NULL
                AND t.deleted_at = '1970-01-01'
                AND pm.privilege_level >= 'readAndWrite'),
     new_parent AS (SELECT t.id, t.path
                    FROM tree_nodes t
                             INNER JOIN node ON t.project_id = node.project_id
                    WHERE t.id = $4
                      AND t.deleted_at = '1970-01-01'
                      AND NOT starts_with(t.path, node.path)),
     updated AS (
         UPDATE tree_nodes t
             SET parent_id = new_parent.id,
                 path = concat(new_parent.path, node.name, '/')
             FROM node, new_parent
             WHERE t.id = node.id
             RETURNING t.path),
     updated_children AS (
         UPDATE tree_nodes t
             SET path = concat(updated.path, substr(t.path, node.old_end))
             FROM node, updated
             WHERE t.project_id = node.project_id
                 AND t.deleted_at = '1970-01-01'
                 AND t.id != node.id
                 AND starts_with(t.path, node.path)
             RETURNING t.id, t.kind, t.path),
     updated_docs AS (SELECT array_agg(id) AS ids, array_agg(path) AS paths
                      FROM updated_children
                      WHERE kind = 'doc'),
     updated_files AS (SELECT array_agg(id) AS ids, array_agg(path) AS paths
                       FROM updated_children
                       WHERE kind = 'file'),
     updated_version AS (
         UPDATE projects p
             SET last_updated_by = $2,
                 last_updated_at = transaction_timestamp(),
                 tree_version = tree_version + 1
             FROM updated
             WHERE p.id = $1
             RETURNING p.tree_version)

SELECT updated_version.tree_version,
       updated_docs.ids,
       updated_docs.paths,
       updated_files.ids,
       updated_files.paths
FROM updated_version,
     updated_docs,
     updated_files
`, projectId, userId, folderId, targetFolderId).
		Scan(&v, &docIds, &docPaths, &fileIds, &filePaths)
	if err != nil {
		return 0, nil, nil, rewritePostgresErr(err)
	}
	docs := make([]Doc, len(docIds))
	for i, id := range docIds {
		docs[i].Id = id
		docs[i].Path = sharedTypes.PathName(docPaths[i])
	}
	files := make([]FileRef, len(fileIds))
	for i, id := range fileIds {
		files[i].Id = id
		files[i].Path = sharedTypes.PathName(filePaths[i])
	}
	return v, docs, files, nil
}

func (m *manager) renameTreeLeaf(ctx context.Context, projectId, userId, nodeId sharedTypes.UUID, kind TreeNodeKind, name sharedTypes.Filename) (sharedTypes.Version, sharedTypes.PathName, error) {
	var treeVersion sharedTypes.Version
	var path sharedTypes.PathName
	return treeVersion, path, m.db.QueryRow(ctx, `
WITH node AS (SELECT t.id, f.path AS parent_path
           FROM tree_nodes t
                    INNER JOIN projects p ON t.project_id = p.id
                    INNER JOIN project_members pm
                               ON (t.project_id = pm.project_id AND
                                   pm.user_id = $2)
           			INNER JOIN tree_nodes f ON t.parent_id = f.id
           WHERE t.id = $3
             AND t.kind = $4
             AND t.project_id = $1
             AND p.deleted_at IS NULL
             AND t.deleted_at = '1970-01-01'
             AND pm.privilege_level >= 'readAndWrite'),
     updated AS (
         UPDATE tree_nodes t
             SET path = concat(node.parent_path, $5::TEXT)
             FROM node
             WHERE t.id = node.id
             RETURNING t.id, t.path)

UPDATE projects p
SET last_updated_by = $2,
    last_updated_at = transaction_timestamp(),
    tree_version    = tree_version + 1
FROM updated
WHERE p.id = $1
RETURNING p.tree_version, updated.path
`, projectId, userId, nodeId, kind, name).Scan(&treeVersion, &path)
}

func (m *manager) RenameDoc(ctx context.Context, projectId, userId sharedTypes.UUID, d *Doc) (sharedTypes.Version, sharedTypes.PathName, error) {
	return m.renameTreeLeaf(ctx, projectId, userId, d.Id, TreeNodeKindDoc, d.Name)
}

func (m *manager) RenameFile(ctx context.Context, projectId, userId sharedTypes.UUID, f *FileRef) (sharedTypes.Version, sharedTypes.PathName, error) {
	return m.renameTreeLeaf(ctx, projectId, userId, f.Id, TreeNodeKindFile, f.Name)
}

func (m *manager) RenameFolder(ctx context.Context, projectId, userId sharedTypes.UUID, f *Folder) (sharedTypes.Version, []Doc, []FileRef, error) {
	var v sharedTypes.Version
	var docIds sharedTypes.UUIDs
	var docPaths []string
	var filesIds sharedTypes.UUIDs
	var filesPaths []string
	err := m.db.QueryRow(ctx, `
WITH node AS (SELECT t.id,
                     t.project_id,
                     t.path,
                     char_length(t.path) + 1            AS old_end,
                     concat(parent.path, $4::TEXT, '/') AS new_path
              FROM tree_nodes t
                       INNER JOIN projects p ON t.project_id = p.id
                       INNER JOIN project_members pm
                                  ON (t.project_id = pm.project_id AND
                                      pm.user_id = $2)
                       INNER JOIN tree_nodes parent ON t.parent_id = parent.id
              WHERE t.id = $3
                AND t.kind = 'folder'
                AND t.project_id = $1
                AND t.parent_id IS NOT NULL
                AND p.deleted_at IS NULL
                AND t.deleted_at = '1970-01-01'
                AND pm.privilege_level >= 'readAndWrite'),
     updated_children AS (
         UPDATE tree_nodes t
             SET path = concat(node.new_path, substr(t.path, node.old_end))
             FROM node
             WHERE t.project_id = node.project_id AND
                   t.deleted_at = '1970-01-01' AND
                   starts_with(t.path, node.path)
             RETURNING t.id, t.kind, t.path),
     updated_docs AS (SELECT array_agg(id) AS ids, array_agg(path) AS paths
                      FROM updated_children
                      WHERE kind = 'doc'),
     updated_files AS (SELECT array_agg(id) AS ids, array_agg(path) AS paths
                       FROM updated_children
                       WHERE kind = 'file'),
     updated_version AS (
         UPDATE projects p
             SET last_updated_by = $2,
                 last_updated_at = transaction_timestamp(),
                 tree_version = tree_version + 1
             FROM node
             WHERE p.id = $1
             RETURNING p.tree_version)

SELECT updated_version.tree_version,
       updated_docs.ids,
       updated_docs.paths,
       updated_files.ids,
       updated_files.paths
FROM updated_version,
     updated_docs,
     updated_files
`, projectId, userId, f.Id, f.Name).
		Scan(&v, &docIds, &docPaths, filesIds, filesPaths)
	if err != nil {
		return 0, nil, nil, rewritePostgresErr(err)
	}
	docs := make([]Doc, len(docIds))
	for i, id := range docIds {
		docs[i].Id = id
		docs[i].Path = sharedTypes.PathName(docPaths[i])
	}
	files := make([]FileRef, len(filesIds))
	for i, id := range filesIds {
		files[i].Id = id
		files[i].Path = sharedTypes.PathName(filesPaths[i])
	}
	return v, docs, files, nil
}

func (m *manager) ArchiveForUser(ctx context.Context, projectId, userId sharedTypes.UUID) error {
	return getErr(m.db.Exec(ctx, `
UPDATE project_members
SET archived = TRUE,
    trashed  = FALSE
WHERE project_id = $1
  AND user_id = $2
`, projectId, userId))
}

func (m *manager) UnArchiveForUser(ctx context.Context, projectId, userId sharedTypes.UUID) error {
	return getErr(m.db.Exec(ctx, `
UPDATE project_members
SET archived = FALSE
WHERE project_id = $1
  AND user_id = $2
`, projectId, userId))
}

func (m *manager) TrashForUser(ctx context.Context, projectId, userId sharedTypes.UUID) error {
	return getErr(m.db.Exec(ctx, `
UPDATE project_members
SET archived = FALSE,
    trashed  = TRUE
WHERE project_id = $1
  AND user_id = $2
`, projectId, userId))
}

func (m *manager) UnTrashForUser(ctx context.Context, projectId, userId sharedTypes.UUID) error {
	return getErr(m.db.Exec(ctx, `
UPDATE project_members
SET trashed = FALSE
WHERE project_id = $1
  AND user_id = $2
`, projectId, userId))
}

var ErrEpochIsNotStable = errors.New("epoch is not stable")

func (m *manager) GetProjectNames(ctx context.Context, userId sharedTypes.UUID) (Names, error) {
	var raw []string
	err := m.db.QueryRow(ctx, `
SELECT array_agg(name)
FROM projects p
         INNER JOIN project_members pm ON p.id = pm.project_id
WHERE user_id = $1
  AND p.deleted_at IS NULL
`, userId).Scan(&raw)
	if err != nil {
		return nil, err
	}
	names := make(Names, len(raw))
	for i, s := range raw {
		names[i] = Name(s)
	}
	return names, nil
}

func (m *manager) GetAuthorizationDetails(ctx context.Context, projectId, userId sharedTypes.UUID, accessToken AccessToken) (*AuthorizationDetails, error) {
	p := &ForAuthorizationDetails{}
	err := m.db.QueryRow(ctx, `
SELECT coalesce(pm.access_source::TEXT, ''),
       coalesce(pm.privilege_level::TEXT, ''),
       p.epoch,
       p.public_access_level,
       coalesce(p.token_ro, ''),
       coalesce(p.token_rw, '')
FROM projects p
         LEFT JOIN project_members pm ON (p.id = pm.project_id AND
                                          pm.user_id = $2)
WHERE p.id = $1
  AND p.deleted_at IS NULL
  AND (
        (pm.access_source >= 'invite') OR
        (p.public_access_level = 'tokenBased' AND
         (pm.access_source = 'token' OR p.token_ro = $3))
    )
`, projectId, userId, accessToken).Scan(
		&p.Member.AccessSource, &p.Member.PrivilegeLevel, &p.Epoch,
		&p.PublicAccessLevel, &p.Tokens.ReadOnly, &p.Tokens.ReadAndWrite,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, &errors.NotAuthorizedError{}
		}
		return nil, err
	}
	return p.GetPrivilegeLevel(userId, accessToken)
}

func (m *manager) GetForProjectInvite(ctx context.Context, projectId, actorId sharedTypes.UUID, email sharedTypes.Email) (*ForProjectInvite, error) {
	d := ForProjectInvite{}
	return &d, m.db.QueryRow(ctx, `
WITH u AS (SELECT id, email, first_name, last_name FROM users WHERE email = $3)
SELECT coalesce(pm.access_source::TEXT, ''),
       coalesce(pm.privilege_level::TEXT, ''),
       p.name,
       p.public_access_level,
       o.id,
       o.email,
       o.first_name,
       o.last_name,
       coalesce(u.id, '00000000-0000-0000-0000-000000000000'::UUID),
       coalesce(u.email, $3),
       coalesce(u.first_name, ''),
       coalesce(u.last_name, '')
FROM projects p
         INNER JOIN users o ON p.owner_id = o.id
         LEFT JOIN u ON TRUE
         LEFT JOIN project_members pm
                   ON (p.id = pm.project_id AND pm.user_id = u.id)

WHERE p.id = $1
  AND p.owner_id = $2
  AND p.deleted_at IS NULL
`, projectId, actorId, email).Scan(
		&d.AccessSource,
		&d.PrivilegeLevel,
		&d.Name,
		&d.PublicAccessLevel,
		&d.Sender.Id,
		&d.Sender.Email,
		&d.Sender.FirstName,
		&d.Sender.LastName,
		&d.User.Id,
		&d.User.Email,
		&d.User.FirstName,
		&d.User.LastName,
	)
}

func (m *manager) GetForProjectJWT(ctx context.Context, projectId, userId sharedTypes.UUID, accessToken AccessToken) (*ForProjectJWT, int64, error) {
	p := ForProjectJWT{}
	var userEpoch int64
	err := m.db.QueryRow(ctx, `
SELECT coalesce(pm.access_source::TEXT, ''),
       coalesce(pm.privilege_level::TEXT, ''),
       p.epoch,
       p.public_access_level,
       coalesce(p.token_ro, ''),
       coalesce(p.token_rw, ''),
       o.features,
       coalesce(u.epoch, 0)
FROM projects p
         INNER JOIN users o ON p.owner_id = o.id
         LEFT JOIN project_members pm ON (p.id = pm.project_id AND
                                          pm.user_id = $2)
         LEFT JOIN users u ON (pm.user_id = u.id AND
                               u.id = $2 AND
                               u.deleted_at IS NULL)
WHERE p.id = $1
  AND p.deleted_at IS NULL
  AND (
        (pm.access_source >= 'invite') OR
        (p.public_access_level = 'tokenBased' AND
         (pm.access_source = 'token' OR p.token_ro = $3))
    )
`, projectId, userId, accessToken).Scan(
		&p.Member.AccessSource,
		&p.Member.PrivilegeLevel,
		&p.Epoch,
		&p.PublicAccessLevel,
		&p.Tokens.ReadOnly,
		&p.Tokens.ReadAndWrite,
		&p.OwnerFeatures,
		&userEpoch,
	)
	if err != nil {
		return nil, 0, err
	}
	return &p, userEpoch, err
}

func (m *manager) ValidateProjectJWTEpochs(ctx context.Context, projectId, userId sharedTypes.UUID, projectEpoch, userEpoch int64) error {
	ok := false
	var err error
	if userId.IsZero() {
		err = m.db.QueryRow(ctx, `
SELECT TRUE
FROM projects
WHERE id = $1 AND epoch = $2
`, projectId, projectEpoch).Scan(&ok)
	} else {
		err = m.db.QueryRow(ctx, `
SELECT TRUE
FROM projects p, users u
WHERE p.id = $1 AND p.epoch = $2 AND u.id = $3 AND u.epoch = $4
`, projectId, projectEpoch, userId, userEpoch).Scan(&ok)
	}
	if err != nil && err != pgx.ErrNoRows {
		return err
	}
	if err == nil && ok {
		return nil
	}
	return &errors.UnauthorizedError{Reason: "epoch mismatch"}
}

func (m *manager) GetDoc(ctx context.Context, projectId, docId sharedTypes.UUID) (*Doc, error) {
	d := Doc{}
	err := m.db.QueryRow(ctx, `
SELECT t.path, d.snapshot, d.version
FROM docs d
         INNER JOIN tree_nodes t ON d.id = t.id
         INNER JOIN projects p ON t.project_id = p.id
WHERE d.id = $2
  AND t.project_id = $1
  AND t.deleted_at = '1970-01-01'
  AND p.deleted_at IS NULL
`, projectId, docId).Scan(&d.Path, &d.Snapshot, &d.Version)
	if err == pgx.ErrNoRows {
		return nil, &errors.DocNotFoundError{}
	}
	d.Id = docId
	d.Name = d.Path.Filename()
	return &d, err
}

func (m *manager) RestoreDoc(ctx context.Context, projectId, userId, docId sharedTypes.UUID, name sharedTypes.Filename) (sharedTypes.Version, sharedTypes.UUID, error) {
	var v sharedTypes.Version
	var rootFolderId sharedTypes.UUID
	return v, rootFolderId, rewritePostgresErr(m.db.QueryRow(ctx, `
WITH d AS (SELECT t.id, p.root_folder_id
           FROM tree_nodes t
                    INNER JOIN projects p ON t.project_id = p.id
                    INNER JOIN project_members pm
                               ON (t.project_id = pm.project_id AND
                                   pm.user_id = $2)
           WHERE t.id = $3
             AND t.kind = 'doc'
             AND t.project_id = $1
             AND p.deleted_at IS NULL
             AND t.deleted_at != '1970-01-01'
             AND pm.privilege_level >= 'readAndWrite'),
     restored
         AS (
         UPDATE tree_nodes t
             SET deleted_at = '1970-01-01',
                 parent_id = d.root_folder_id,
                 path = $4
             FROM d
             WHERE t.id = d.id
             RETURNING t.id, t.parent_id)

UPDATE projects p
SET last_updated_by = $2,
    last_updated_at = transaction_timestamp(),
    tree_version    = tree_version + 1
FROM restored
WHERE p.id = $1
RETURNING p.tree_version, restored.parent_id
`, projectId, userId, docId, name).Scan(&v, &rootFolderId))
}

func (m *manager) GetFile(ctx context.Context, projectId, userId sharedTypes.UUID, accessToken AccessToken, fileId sharedTypes.UUID) (*FileWithParent, error) {
	f := FileWithParent{}
	err := m.db.QueryRow(ctx, `
SELECT t.path, t.parent_id, f.linked_file_data, f.size
FROM files f
         INNER JOIN tree_nodes t ON f.id = t.id
         INNER JOIN projects p ON t.project_id = p.id
         LEFT JOIN project_members pm ON (p.id = pm.project_id AND
                                          pm.user_id = $2)
WHERE f.id = $4
  AND t.project_id = $1
  AND t.deleted_at = '1970-01-01'
  AND p.deleted_at IS NULL
  AND (
        (pm.access_source >= 'invite') OR
        (p.public_access_level = 'tokenBased' AND
         (pm.access_source = 'token' OR p.token_ro = $3))
    )
`, projectId, userId, accessToken, fileId).Scan(
		&f.Path, &f.ParentId, &f.LinkedFileData, &f.Size,
	)
	f.Id = fileId
	f.Name = f.Path.Filename()
	return &f, err
}

func (m *manager) getElementHintForOverwrite(ctx context.Context, projectId, userId, folderId sharedTypes.UUID, name sharedTypes.Filename, tx pgx.Tx) (sharedTypes.UUID, bool, error) {
	var nodeId sharedTypes.UUID
	var kind TreeNodeKind
	err := tx.QueryRow(ctx, `
SELECT t.id, t.kind
FROM tree_nodes t
         INNER JOIN projects p ON t.project_id = p.id
         INNER JOIN project_members pm ON (p.id = pm.project_id AND
                                           pm.user_id = $2)
         INNER JOIN tree_nodes f ON t.parent_id = f.id
WHERE t.project_id = $1
  AND f.id = $3
  AND t.deleted_at = '1970-01-01'
  AND p.deleted_at IS NULL
  AND (t.path = $4 OR t.path LIKE concat('%/', $4::TEXT))
`, projectId, userId, folderId, name).Scan(&nodeId, &kind)
	if err == pgx.ErrNoRows {
		return nodeId, false, nil
	}
	if kind == TreeNodeKindFolder {
		return nodeId, false, &errors.UnprocessableEntityError{
			Msg: "element is a folder",
		}
	}
	return nodeId, kind == TreeNodeKindDoc, err
}

func (m *manager) GetElementByPath(ctx context.Context, projectId, userId sharedTypes.UUID, path sharedTypes.PathName) (sharedTypes.UUID, bool, error) {
	var id sharedTypes.UUID
	var isDoc bool
	return id, isDoc, m.db.QueryRow(ctx, `
SELECT t.id, t.kind = 'doc'
FROM tree_nodes t
         INNER JOIN projects p ON (t.project_id = p.id)
         INNER JOIN project_members pm ON (t.project_id = pm.project_id AND
                                           pm.user_id = $2)
WHERE t.project_id = $1
  AND p.deleted_at IS NULL
  AND t.deleted_at = '1970-01-01'
  AND t.path = $3
  AND (t.kind = 'doc' OR t.kind = 'file')
`, projectId, userId, path).Scan(&id, &isDoc)
}

func (m *manager) GetProjectWithContent(ctx context.Context, projectId sharedTypes.UUID) ([]Doc, []FileRef, error) {
	r, err := m.db.Query(ctx, `
SELECT t.id, t.path, coalesce(d.snapshot, ''), coalesce(d.version, -1)
FROM tree_nodes t
         INNER JOIN projects p ON t.project_id = p.id
         LEFT JOIN docs d ON t.id = d.id
WHERE t.project_id = $1
  AND p.deleted_at IS NULL
  AND t.deleted_at = '1970-01-01'
  -- Get all files, docs and also the root folder to differentiate between
  --  and empty tree and a missing project.
  AND ((t.kind = 'doc' OR t.kind = 'file') OR t.parent_id IS NULL)
ORDER BY t.kind
`, projectId)
	if err != nil {
		return nil, nil, err
	}
	defer r.Close()
	nodes := make([]Doc, 0)
	for i := 0; r.Next(); i++ {
		nodes = append(nodes, Doc{})
		err = r.Scan(
			&nodes[i].Id, &nodes[i].Path, &nodes[i].Snapshot,
			&nodes[i].Version,
		)
		if err != nil {
			return nil, nil, err
		}
	}
	if err = r.Err(); err != nil {
		return nil, nil, err
	}
	if len(nodes) == 0 {
		return nil, nil, &errors.NotFoundError{}
	}
	// drop root folder
	nodes = nodes[:len(nodes)-1]

	var files []FileRef
	for _, d := range nodes {
		if d.Version == -1 {
			files = append(files, FileRef{
				LeafFields: d.LeafFields,
			})
		}
	}
	return nodes[:len(nodes)-len(files)], files, nil
}

func (m *manager) GetForZip(ctx context.Context, projectId sharedTypes.UUID, userId sharedTypes.UUID, accessToken AccessToken) (*ForZip, error) {
	p := ForZip{}
	return &p, m.db.QueryRow(ctx, `
WITH tree AS
         (SELECT t.project_id,
                 array_agg(t.id)                     AS ids,
                 array_agg(t.kind::TEXT)             AS kinds,
                 array_agg(t.path)                   AS paths,
                 array_agg(coalesce(d.snapshot, '')) AS snapshots
          FROM tree_nodes t
                   LEFT JOIN docs d ON t.id = d.id
          WHERE t.project_id = $1
            AND t.deleted_at = '1970-01-01'
            AND t.parent_id IS NOT NULL
          GROUP BY t.project_id)

SELECT p.name,
       tree.ids,
       tree.kinds,
       tree.paths,
       tree.snapshots
FROM projects p
         LEFT JOIN tree ON (p.id = tree.project_id)
         LEFT JOIN project_members pm ON (p.id = pm.project_id AND
                                          pm.user_id = $2)
WHERE p.id = $1
  AND p.deleted_at IS NULL
  AND (
        (pm.access_source >= 'invite') OR
        (p.public_access_level = 'tokenBased' AND
         (pm.access_source = 'token' OR p.token_ro = $3))
    )
`, projectId, userId, accessToken).Scan(
		&p.Name,
		&p.treeIds,
		&p.treeKinds,
		&p.treePaths,
		&p.docSnapshots,
	)
}

func (m *manager) GetBootstrapWSDetails(ctx context.Context, projectId, userId sharedTypes.UUID, projectEpoch, userEpoch int64, source AccessSource) (*GetBootstrapWSDetails, error) {
	d := &GetBootstrapWSDetails{}
	d.Project.Id = projectId
	d.Project.RootFolder = NewFolder("")
	d.Project.DeletedDocs = make([]CommonTreeFields, 0)

	var deletedDocIds sharedTypes.UUIDs
	var deletedDocNames []string

	err := m.db.QueryRow(ctx, `
WITH tree AS
         (SELECT t.project_id,
                 array_agg(t.id)                AS ids,
                 array_agg(t.kind::TEXT)        AS kinds,
                 array_agg(t.path)              AS paths,
                 array_agg(t.created_at)        AS created_ats,
                 array_agg(f.linked_file_data)  AS linked_file_data,
                 array_agg(coalesce(f.size, 0)) AS sizes
          FROM tree_nodes t
                   LEFT JOIN files f ON t.id = f.id
          WHERE t.project_id = $1
            AND t.deleted_at = '1970-01-01'
            AND t.parent_id IS NOT NULL
          GROUP BY t.project_id),
     deleted_docs AS (SELECT t.project_id,
                             array_agg(t.id)                        AS ids,
                             array_agg(split_part(t.path, '/', -1)) AS names
                      FROM tree_nodes t
                      WHERE t.project_id = $1
                        AND t.deleted_at != '1970-01-01'
                      GROUP BY t.project_id)

SELECT p.compiler,
       p.image_name,
       p.name,
       p.owner_id,
       p.public_access_level,
       coalesce(p.root_doc_id, '00000000-0000-0000-0000-000000000000'::UUID),
       p.root_folder_id,
       p.spell_check_language,
       p.tree_version,
       coalesce(o.email, ''),
       coalesce(o.first_name, ''),
       coalesce(o.last_name, ''),
       coalesce(u.email, ''),
       coalesce(u.epoch, 0),
       coalesce(u.first_name, ''),
       coalesce(u.last_name, ''),
       tree.ids,
       tree.kinds,
       tree.paths,
       tree.created_ats,
       tree.linked_file_data,
       tree.sizes,
       deleted_docs.ids,
       deleted_docs.names
FROM projects p
         LEFT JOIN users o
                   ON (p.owner_id = o.id AND $5::AccessSource > 'token')
         LEFT JOIN tree ON (p.id = tree.project_id)
         LEFT JOIN deleted_docs ON (p.id = deleted_docs.project_id)
         LEFT JOIN project_members pm ON (p.id = pm.project_id AND
                                          pm.user_id = $2)
         LEFT JOIN users u ON (pm.user_id = u.id AND
                               u.id = $2 AND
                               u.epoch = $4 AND
                               u.deleted_at IS NULL)
WHERE p.id = $1
  AND p.deleted_at IS NULL
  AND p.epoch = $3
`, projectId, userId, projectEpoch, userEpoch, source).Scan(
		&d.Project.Compiler,
		&d.Project.ImageName,
		&d.Project.Name,
		&d.Project.Owner.Id,
		&d.Project.PublicAccessLevel,
		&d.Project.RootDocId,
		&d.Project.RootFolder.Id,
		&d.Project.SpellCheckLanguage,
		&d.Project.Version,
		&d.Project.Owner.Email,
		&d.Project.Owner.FirstName,
		&d.Project.Owner.LastName,
		&d.User.Email,
		&d.User.Epoch,
		&d.User.FirstName,
		&d.User.LastName,
		&d.Project.treeIds,
		&d.Project.treeKinds,
		&d.Project.treePaths,
		&d.Project.createdAts,
		&d.Project.linkedFileData,
		&d.Project.sizes,
		&deletedDocIds,
		&deletedDocNames,
	)
	if err == pgx.ErrNoRows || (!userId.IsZero() && d.User.Epoch != userEpoch) {
		return nil, &errors.UnauthorizedError{}
	}
	if err != nil {
		return nil, err
	}
	if n := len(deletedDocIds); n > 0 {
		d.Project.DeletedDocs = make([]CommonTreeFields, n)
		for i, id := range deletedDocIds {
			d.Project.DeletedDocs[i].Id = id
			d.Project.DeletedDocs[i].Name = sharedTypes.Filename(
				deletedDocNames[i],
			)
		}
	}
	return d, nil
}

func (m *manager) BumpLastOpened(ctx context.Context, projectId sharedTypes.UUID) error {
	return getErr(m.db.Exec(ctx, `
UPDATE projects
SET last_opened_at = transaction_timestamp()
WHERE id = $1
`, projectId))
}

func (m *manager) GetLoadEditorDetails(ctx context.Context, projectId, userId sharedTypes.UUID, accessToken AccessToken) (*LoadEditorDetails, error) {
	d := LoadEditorDetails{}
	err := m.db.QueryRow(ctx, `
SELECT coalesce(pm.access_source::TEXT, ''),
       coalesce(pm.privilege_level::TEXT, ''),
       p.compiler,
       p.epoch,
       p.image_name,
       p.name,
       p.public_access_level,
       coalesce(p.token_ro, ''),
       coalesce(p.token_rw, ''),
       p.tree_version,
       coalesce(d.id, '00000000-0000-0000-0000-000000000000'::UUID),
       coalesce(d.path, ''),
       o.features,
       coalesce(u.editor_config, '{}'),
       coalesce(u.email, ''),
       coalesce(u.epoch, 0),
       coalesce(u.first_name, ''),
       coalesce(u.last_name, ''),
       coalesce(u.learned_words, ARRAY []::TEXT[])
FROM projects p
         INNER JOIN users o ON p.owner_id = o.id
         LEFT JOIN tree_nodes d ON p.root_doc_id = d.id
         LEFT JOIN project_members pm ON (p.id = pm.project_id AND
                                          pm.user_id = $2)
         LEFT JOIN users u ON (pm.user_id = u.id AND
                               u.id = $2 AND
                               u.deleted_at IS NULL)
WHERE p.id = $1
  AND p.deleted_at IS NULL
  AND (
        (pm.access_source >= 'invite') OR
        (p.public_access_level = 'tokenBased' AND
         (pm.access_source = 'token' OR p.token_ro = $3))
    )
`, projectId, userId, accessToken).Scan(
		&d.Project.Member.AccessSource,
		&d.Project.Member.PrivilegeLevel,
		&d.Project.Compiler,
		&d.Project.Epoch,
		&d.Project.ImageName,
		&d.Project.Name,
		&d.Project.PublicAccessLevel,
		&d.Project.Tokens.ReadOnly,
		&d.Project.Tokens.ReadAndWrite,
		&d.Project.Version,
		&d.Project.RootDoc.Id,
		&d.Project.RootDoc.Path,
		&d.Project.OwnerFeatures,
		&d.User.EditorConfig,
		&d.User.Email,
		&d.User.Epoch,
		&d.User.FirstName,
		&d.User.LastName,
		&d.User.LearnedWords,
	)
	if err != nil {
		return nil, err
	}
	d.Project.Id = projectId
	d.User.Id = userId
	d.Project.RootDocId = d.Project.RootDoc.Id
	return &d, err
}

func (m *manager) GetLastUpdatedAt(ctx context.Context, projectId sharedTypes.UUID) (time.Time, error) {
	at := time.Time{}
	return at, m.db.QueryRow(ctx, `
SELECT last_updated_at
FROM projects
WHERE id = $1 AND deleted_at IS NULL
`, projectId).Scan(&at)
}

func (m *manager) GetForClone(ctx context.Context, projectId, userId sharedTypes.UUID) (*ForClone, error) {
	p := ForClone{}
	return &p, m.db.QueryRow(ctx, `
WITH tree AS
         (SELECT t.project_id,
                 array_agg(t.id)                     AS ids,
                 array_agg(t.kind::TEXT)             AS kinds,
                 array_agg(t.path)                   AS paths,
                 array_agg(coalesce(d.snapshot, '')) AS snapshots,
                 array_agg(t.created_at)             AS created_ats,
                 array_agg(f.linked_file_data)       AS linked_file_data,
                 array_agg(coalesce(f.size, 0))      AS sizes
          FROM tree_nodes t
                   LEFT JOIN docs d ON t.id = d.id
                   LEFT JOIN files f ON t.id = f.id
          WHERE t.project_id = $1
            AND t.deleted_at = '1970-01-01'
            AND t.parent_id IS NOT NULL
          GROUP BY t.project_id)

SELECT p.compiler,
       p.image_name,
       coalesce(p.root_doc_id, '00000000-0000-0000-0000-000000000000'::UUID),
       p.spell_check_language,
       tree.ids,
       tree.kinds,
       tree.paths,
       tree.snapshots,
       tree.created_ats,
       tree.linked_file_data,
       tree.sizes
FROM projects p
         INNER JOIN project_members pm ON (p.id = pm.project_id AND
                                           pm.user_id = $2)
         LEFT JOIN tree ON (p.id = tree.project_id)

WHERE p.id = $1
  AND p.deleted_at IS NULL
  AND (
        (pm.access_source >= 'invite') OR
        (p.public_access_level = 'tokenBased' AND pm.access_source = 'token')
    )
`, projectId, userId).Scan(
		&p.Compiler,
		&p.ImageName,
		&p.RootDoc.Id,
		&p.SpellCheckLanguage,
		&p.treeIds,
		&p.treeKinds,
		&p.treePaths,
		&p.docSnapshots,
		&p.createdAts,
		&p.linkedFileData,
		&p.sizes,
	)
}

type TreeEntity struct {
	Path string `json:"path"`
	Type string `json:"type"`
}

func (m *manager) GetTreeEntities(ctx context.Context, projectId, userId sharedTypes.UUID) ([]TreeEntity, error) {
	r, err := m.db.Query(ctx, `
SELECT path, kind
FROM tree_nodes t
         INNER JOIN projects p ON t.project_id = p.id
         INNER JOIN project_members pm ON (p.id = pm.project_id AND
                                           pm.user_id = $2)
WHERE t.project_id = $1
  AND p.deleted_at IS NULL
  AND t.deleted_at = '1970-01-01'
  AND (t.kind = 'doc' OR t.kind = 'file')
`, projectId, userId)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	entries := make([]TreeEntity, 0)
	for i := 0; r.Next(); i++ {
		entries = append(entries, TreeEntity{})
		err = r.Scan(&entries[i].Path, &entries[i].Type)
		if err != nil {
			return nil, err
		}
	}
	if err = r.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func (m *manager) GetProjectMembers(ctx context.Context, projectId sharedTypes.UUID) ([]user.AsProjectMember, error) {
	r, err := m.db.Query(ctx, `
SELECT u.id,
       u.email,
       u.first_name,
       u.last_name,
       pm.privilege_level
FROM project_members pm
         INNER JOIN projects p ON p.id = pm.project_id
         INNER JOIN users u ON pm.user_id = u.id
WHERE p.id = $1
  AND p.deleted_at IS NULL
  AND pm.access_source = 'invite'
  AND u.deleted_at IS NULL
`, projectId)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	c := make([]user.AsProjectMember, 0)
	for i := 0; r.Next(); i++ {
		c = append(c, user.AsProjectMember{})
		err = r.Scan(
			&c[i].Id, &c[i].Email, &c[i].FirstName, &c[i].LastName,
			&c[i].PrivilegeLevel,
		)
		if err != nil {
			return nil, err
		}
	}
	if err = r.Err(); err != nil {
		return nil, err
	}
	return c, nil
}

func (m *manager) GrantMemberAccess(ctx context.Context, projectId, ownerId, userId sharedTypes.UUID, privilegeLevel sharedTypes.PrivilegeLevel) error {
	return getErr(m.db.Exec(ctx, `
UPDATE project_members pm
SET privilege_level = $4
FROM projects p
WHERE p.id = $1
  AND p.owner_id = $2
  AND p.id = pm.project_id
  AND pm.user_id = $3
  AND pm.access_source = 'invite'
`, projectId, ownerId, userId, privilegeLevel))
}

func (m *manager) GetTokenAccessDetails(ctx context.Context, userId sharedTypes.UUID, privilegeLevel sharedTypes.PrivilegeLevel, accessToken AccessToken) (*ForTokenAccessDetails, *AuthorizationDetails, error) {
	p := ForTokenAccessDetails{}
	q, err := accessToken.toQueryParameters(privilegeLevel)
	if err != nil {
		return nil, nil, err
	}
	err = m.db.QueryRow(ctx, `
SELECT coalesce(pm.access_source::TEXT, ''),
       coalesce(pm.privilege_level::TEXT, ''),
       p.id,
       p.epoch,
       p.name,
       coalesce(p.token_ro, ''),
       coalesce(p.token_rw, '')
FROM projects p
         LEFT JOIN project_members pm ON (p.id = pm.project_id AND
                                          pm.user_id = $1)
WHERE (p.token_ro = $2 OR p.token_rw_prefix = $3)
  AND p.public_access_level = 'tokenBased'
  AND p.deleted_at IS NULL
`, userId, q.tokenRO, q.tokenRWPrefix).Scan(
		&p.Member.AccessSource, &p.Member.PrivilegeLevel,
		&p.Id, &p.Epoch, &p.Name, &p.Tokens.ReadOnly, &p.Tokens.ReadAndWrite,
	)
	if err != nil {
		return nil, nil, err
	}
	p.PublicAccessLevel = TokenBasedAccess
	// Ensure that the token_rw is compared in full as the db lookup is by
	//  token_rw_prefix only.
	d, err := p.GetPrivilegeLevelAnonymous(accessToken)
	if err != nil {
		return nil, nil, err
	}
	return &p, d, nil
}

func (m *manager) GrantTokenAccess(ctx context.Context, projectId, userId sharedTypes.UUID, accessToken AccessToken, privilegeLevel sharedTypes.PrivilegeLevel) error {
	q, err := accessToken.toQueryParameters(privilegeLevel)
	if err != nil {
		return err
	}
	return getErr(m.db.Exec(ctx, `
INSERT INTO project_members
(project_id, user_id, access_source, privilege_level, archived, trashed)
SELECT p.id, $2, 'token', $5, FALSE, FALSE
FROM projects p
WHERE id = $1
  AND deleted_at IS NULL
  AND public_access_level = 'tokenBased'
  AND (token_ro = $3 OR token_rw_prefix = $4)

ON CONFLICT (project_id, user_id)
WHERE privilege_level < $5
    DO
UPDATE
SET privilege_level = $5
`, projectId, userId, q.tokenRO, q.tokenRWPrefix, privilegeLevel))
}

func (m *manager) RemoveMember(ctx context.Context, projectId sharedTypes.UUID, actor, userId sharedTypes.UUID) error {
	return getErr(m.db.Exec(ctx, `
WITH pm AS (
    DELETE FROM project_members pm
        USING projects p
        WHERE pm.project_id = $1
            AND pm.user_id = $3
            AND p.owner_id != $3
            AND (p.owner_id = $2 OR $2 = $3)
        RETURNING project_id)
UPDATE projects
SET epoch = epoch + 1
FROM pm
WHERE id = pm.project_id
`, projectId, actor, userId))
}

func (m *manager) SoftDelete(ctx context.Context, projectIds sharedTypes.UUIDs, userId sharedTypes.UUID, ipAddress string) error {
	blob, err := json.Marshal(map[string]string{
		"ipAddress": ipAddress,
	})
	if err != nil {
		return err
	}
	r, err := m.db.Exec(ctx, `
WITH soft_deleted AS (
    UPDATE projects
        SET deleted_at = transaction_timestamp(),
            epoch = epoch + 1
        WHERE id = ANY ($1) AND owner_id = $2 AND deleted_at IS NULL
        RETURNING id)

INSERT
INTO project_audit_log
(created_at, id, info, initiator_id, operation, project_id)
SELECT transaction_timestamp(),
       gen_random_uuid(),
       $3,
       $2,
       'soft-deletion',
       id
FROM soft_deleted
`, projectIds, userId, blob)
	if err != nil {
		return err
	}
	if r.RowsAffected() != int64(len(projectIds)) {
		return errors.New("incomplete soft deletion")
	}
	return nil
}

func (m *manager) HardDelete(ctx context.Context, projectId sharedTypes.UUID) error {
	r, err := m.db.Exec(ctx, `
DELETE
FROM projects
WHERE id = $1
  AND deleted_at IS NOT NULL
`, projectId)
	if err != nil {
		return err
	}
	if r.RowsAffected() == 0 {
		return &errors.UnprocessableEntityError{
			Msg: "user missing or not deleted",
		}
	}
	return nil
}

func (m *manager) ProcessSoftDeleted(ctx context.Context, cutOff time.Time, fn func(projectId sharedTypes.UUID) bool) error {
	ids := make(sharedTypes.UUIDs, 0, 100)
	for {
		ids = ids[:0]
		r := m.db.QueryRow(ctx, `
WITH ids AS (SELECT id
             FROM projects
             WHERE deleted_at <= $1
             ORDER BY deleted_at
             LIMIT 100)
SELECT array_agg(ids.id)
FROM ids
`, cutOff)
		if err := r.Scan(&ids); err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		ok := true
		for _, projectId := range ids {
			if !fn(projectId) {
				ok = false
			}
		}
		if !ok {
			return nil
		}
	}
}

func (m *manager) GetDeletedProjectsName(ctx context.Context, projectId, userId sharedTypes.UUID) (Name, error) {
	var name Name
	return name, m.db.QueryRow(ctx, `
SELECT p.name
FROM projects p
         INNER JOIN users u ON p.owner_id = u.id
WHERE p.id = $1
  AND p.deleted_at IS NOT NULL
  AND u.id = $2
  AND u.deleted_at IS NULL
`, projectId, userId).Scan(&name)
}

func (m *manager) Restore(ctx context.Context, projectId, userId sharedTypes.UUID, name Name) error {
	return getErr(m.db.Exec(ctx, `
UPDATE projects p
SET deleted_at = NULL,
    epoch      = epoch + 1,
    name       = $3
FROM users u
WHERE p.id = $1
  AND p.deleted_at IS NOT NULL
  AND p.owner_id = $2
  AND p.owner_id = u.id
  AND u.deleted_at IS NULL
`, projectId, userId, name))
}

func (m *manager) CreateDoc(ctx context.Context, projectId, userId, folderId sharedTypes.UUID, d *Doc) (sharedTypes.Version, error) {
	return m.createDocVia(ctx, projectId, userId, folderId, d, m.db)
}

func (m *manager) createDocVia(ctx context.Context, projectId, userId, folderId sharedTypes.UUID, d *Doc, runner queryRunner) (sharedTypes.Version, error) {
	var v sharedTypes.Version
	return v, rewritePostgresErr(runner.QueryRow(ctx, `
WITH f AS (SELECT t.id, t.path
           FROM tree_nodes t
                    INNER JOIN projects p ON t.project_id = p.id
                    INNER JOIN project_members pm
                               ON (t.project_id = pm.project_id AND
                                   pm.user_id = $2)
           WHERE t.id = $3
             AND t.project_id = $1
             AND p.deleted_at IS NULL
             AND t.deleted_at = '1970-01-01'
             AND pm.privilege_level >= 'readAndWrite'),
     inserted_tree_node AS (
         INSERT INTO tree_nodes
             (created_at, deleted_at, id, kind, parent_id, path, project_id)
             SELECT transaction_timestamp(),
                    '1970-01-01',
                    $4,
                    'doc',
                    f.id,
                    concat(f.path, $5::TEXT),
                    $1
             FROM f
             RETURNING id),
     inserted_doc AS (
         INSERT INTO docs
             (id, snapshot, version)
             SELECT inserted_tree_node.id, $6, 0
             FROM inserted_tree_node
             RETURNING FALSE)

UPDATE projects p
SET last_updated_by = $2,
    last_updated_at = transaction_timestamp(),
    tree_version    = tree_version + 1
FROM inserted_doc
WHERE p.id = $1
RETURNING p.tree_version
`, projectId, userId, folderId, d.Id, d.Name, d.Snapshot).Scan(&v))
}

func (m *manager) EnsureIsDoc(ctx context.Context, projectId, userId, folderId sharedTypes.UUID, d *Doc) (sharedTypes.UUID, bool, sharedTypes.Version, error) {
	v, errInsert := m.CreateDoc(ctx, projectId, userId, folderId, d)
	if errInsert == nil || errInsert != ErrDuplicateNameInFolder {
		return sharedTypes.UUID{}, false, 0, errInsert
	}
	tx, err := m.db.Begin(ctx)
	if err != nil {
		return sharedTypes.UUID{}, false, 0, errors.Tag(err, "start tx")
	}
	ok := false
	defer func() {
		if !ok {
			_ = tx.Rollback(ctx)
		}
	}()
	existingId, isDoc, err := m.getElementHintForOverwrite(
		ctx, projectId, userId, folderId, d.Name, tx,
	)
	if err != nil {
		return sharedTypes.UUID{}, false, 0, err
	}
	if existingId.IsZero() {
		return existingId, false, 0, errInsert
	}
	if isDoc {
		return existingId, true, v, nil
	}
	_, err = m.deleteTreeLeaf(ctx, projectId, userId, existingId, TreeNodeKindFile, tx)
	if err != nil {
		return existingId, false, 0, errors.Tag(err, "delete existing file")
	}
	v, err = m.createDocVia(ctx, projectId, userId, folderId, d, tx)
	if err != nil {
		return existingId, false, 0, errors.Tag(err, "create doc")
	}
	if err = tx.Commit(ctx); err != nil {
		return existingId, false, 0, errors.Tag(err, "commit tx")
	}
	ok = true
	return existingId, false, v, nil
}

func (m *manager) PrepareFileCreation(ctx context.Context, projectId, userId, folderId sharedTypes.UUID, f *FileRef) error {
	return getErr(m.db.Exec(ctx, `
WITH f AS (SELECT t.id, t.path
           FROM tree_nodes t
                    INNER JOIN projects p ON t.project_id = p.id
                    INNER JOIN project_members pm
                               ON (t.project_id = pm.project_id AND
                                   pm.user_id = $2)
           WHERE t.id = $3
             AND t.project_id = $1
             AND p.deleted_at IS NULL
             AND t.deleted_at = '1970-01-01'
             AND pm.privilege_level >= 'readAndWrite'),
     inserted_tree_node AS (
         INSERT INTO tree_nodes
             (created_at, deleted_at, id, kind, parent_id, path, project_id)
             SELECT $6,
                    $7,
                    $4,
                    'file',
                    f.id,
                    concat(f.path, $5::TEXT),
                    $1
             FROM f
             RETURNING id)
INSERT
INTO files
    (id, hash, linked_file_data, size, pending)
SELECT inserted_tree_node.id, $8, $9, $10, TRUE
FROM inserted_tree_node
`,
		projectId, userId, folderId,
		f.Id, f.Name, f.CreatedAt, f.CreatedAt.Add(-time.Microsecond), f.Hash,
		f.LinkedFileData, f.Size,
	))
}

func (m *manager) FinalizeFileCreation(ctx context.Context, projectId, userId sharedTypes.UUID, f *FileRef) (sharedTypes.UUID, bool, sharedTypes.Version, error) {
	var nodeId sharedTypes.UUID
	var kind TreeNodeKind
	var v sharedTypes.Version
	err := m.db.QueryRow(ctx, `
WITH f AS (SELECT t.id, t.project_id, t.path
           FROM files f
                    INNER JOIN tree_nodes t ON f.id = t.id
                    INNER JOIN projects p ON t.project_id = p.id
                    INNER JOIN project_members pm
                               ON (t.project_id = pm.project_id AND
                                   pm.user_id = $2)
           WHERE f.pending = TRUE
             AND t.id = $3
             AND t.project_id = $1
             AND p.deleted_at IS NULL
             AND t.deleted_at = $4
             AND pm.privilege_level >= 'readAndWrite'),
     d AS (
         UPDATE tree_nodes t
             SET deleted_at = transaction_timestamp()
             FROM f
             WHERE t.project_id = f.project_id
                 AND t.path = f.path
                 AND t.deleted_at = '1970-01-01'
                 AND t.kind != 'folder'
             RETURNING t.id, kind),
     deleted AS (SELECT coalesce(
                                (SELECT id FROM d),
                                '00000000-0000-0000-0000-000000000000'::UUID
                            )                                    AS id,
                        coalesce((SELECT kind FROM d)::TEXT, '') AS kind),
     createdTn AS (
         UPDATE tree_nodes t
             SET deleted_at = '1970-01-01'
             FROM deleted, f
             WHERE t.id = f.id
             RETURNING t.id),
     createdF AS (
         UPDATE files f
             SET pending = FALSE
             FROM createdTn
             WHERE f.id = createdTn.id
             RETURNING FALSE)

UPDATE projects p
SET last_updated_by = $2,
    last_updated_at = transaction_timestamp(),
    tree_version    = tree_version + 1,
    root_doc_id     = NULLIF(root_doc_id, deleted.id)
FROM createdF,
     deleted,
     f
WHERE p.id = f.project_id
RETURNING deleted.id, deleted.kind, p.tree_version
`, projectId, userId, f.Id, f.CreatedAt.Add(-time.Microsecond)).
		Scan(&nodeId, &kind, &v)
	return nodeId, kind == TreeNodeKindDoc, v, err
}

func (m *manager) ProcessStaleFileUploads(ctx context.Context, cutOff time.Time, fn func(projectId, fileId sharedTypes.UUID) bool) error {
	for {
		r, err := m.db.Query(ctx, `
SELECT p.id, f.id
FROM files f
         INNER JOIN tree_nodes t ON f.id = t.id
         INNER JOIN projects p ON t.project_id = p.id
WHERE f.pending = TRUE
  AND t.deleted_at <= $1
ORDER BY t.deleted_at, t.id
LIMIT 100
`, cutOff)
		if err != nil {
			return errors.Tag(err, "get cursor")
		}
		ok := true
		foundAny := false
		var projectId, fileId sharedTypes.UUID
		for r.Next() {
			if err = r.Scan(&projectId, &fileId); err != nil {
				return errors.Tag(err, "deserialize ids")
			}
			foundAny = true
			if !fn(projectId, fileId) {
				ok = false
			}
		}
		if !ok || !foundAny {
			return nil
		}
	}
}

func (m *manager) PurgeStaleFileUpload(ctx context.Context, projectId, fileId sharedTypes.UUID) error {
	return getErr(m.db.Exec(ctx, `
DELETE
FROM tree_nodes t USING files f
WHERE t.project_id = $1
  AND t.id = $2
  AND t.id = f.id
  AND f.pending = TRUE
`, projectId, fileId))
}

func (m *manager) GetOwnedProjects(ctx context.Context, userId sharedTypes.UUID) ([]sharedTypes.UUID, error) {
	ids := make([]sharedTypes.UUID, 0)
	return ids, m.db.QueryRow(ctx, `
SELECT array_agg(p.id)
FROM projects p
         INNER JOIN project_members pm ON p.id = pm.project_id
WHERE pm.user_id = $1
  AND p.deleted_at IS NULL
`, userId).Scan(&ids)
}

func (m *manager) ListProjectsWithName(ctx context.Context, userId sharedTypes.UUID) ([]WithIdAndName, error) {
	r, err := m.db.Query(ctx, `
SELECT p.id, name
FROM projects p
         INNER JOIN project_members pm ON p.id = pm.project_id
WHERE pm.user_id = $1
  AND p.deleted_at IS NULL
`, userId)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	projects := make([]WithIdAndName, 0)
	for i := 0; r.Next(); i++ {
		projects = append(projects, WithIdAndName{})
		if err = r.Scan(&projects[i].Id, &projects[i].Name); err != nil {
			return nil, err
		}
	}
	return projects, r.Err()
}

type ForProjectList struct {
	User          user.ProjectListViewCaller
	Tags          []tag.Full
	Notifications notification.Notifications
	Projects      List
}

func (m *manager) GetProjectListDetails(ctx context.Context, userId sharedTypes.UUID, d *ForProjectList) error {
	b := pgx.Batch{}

	b.Queue(`
SELECT access_source,
       archived,
       p.epoch,
       p.id,
       last_updated_at,
       coalesce(last_updated_by, '00000000-0000-0000-0000-000000000000'::UUID),
       coalesce(l.email, ''),
       coalesce(l.first_name, ''),
       coalesce(l.last_name, ''),
       name,
       owner_id,
       o.email,
       o.first_name,
       o.last_name,
       privilege_level,
       public_access_level,
       trashed,
       (SELECT array_agg(t.tag_id)
        FROM tag_entries t
        WHERE t.project_id = p.id)
FROM projects p
         INNER JOIN project_members pm ON p.id = pm.project_id
         INNER JOIN users o ON p.owner_id = o.id
         LEFT JOIN users l ON (p.last_updated_by = l.id AND
                               l.deleted_at IS NULL)
WHERE pm.user_id = $1
  AND p.deleted_at IS NULL
`, userId)

	b.Queue(`
WITH t AS (SELECT array_agg(id) AS ids, array_agg(name) AS names
           FROM tags
           WHERE user_id = $1),
     n AS (SELECT array_agg(id)              AS ids,
                  array_agg(key)             AS keys,
                  array_agg(template_key)    AS template_keys,
                  array_agg(message_options) AS message_options
           FROM notifications
           WHERE user_id = $1
             AND template_key != ''
             AND expires_at > transaction_timestamp())

SELECT email,
       email_confirmed_at,
       first_name,
       last_name,
       t.ids,
       t.names,
       n.ids,
       n.keys,
       n.template_keys,
       n.message_options
FROM users u,
     t,
     n
WHERE u.id = $1
  AND u.deleted_at IS NULL
`, userId)

	br := m.db.SendBatch(ctx, &b)
	defer func() { _ = br.Close() }()

	r, err := br.Query()
	if err != nil {
		return errors.Tag(err, "wait for projects listing")
	}
	defer r.Close()
	projects := make(List, 0)
	for i := 0; r.Next(); i++ {
		projects = append(projects, ListViewPrivate{})
		err = r.Scan(
			&projects[i].AccessSource,
			&projects[i].Archived,
			&projects[i].Epoch,
			&projects[i].Id,
			&projects[i].LastUpdatedAt,
			&projects[i].LastUpdatedBy,
			&projects[i].LastUpdater.Email,
			&projects[i].LastUpdater.FirstName,
			&projects[i].LastUpdater.LastName,
			&projects[i].Name,
			&projects[i].OwnerId,
			&projects[i].Owner.Email,
			&projects[i].Owner.FirstName,
			&projects[i].Owner.LastName,
			&projects[i].PrivilegeLevel,
			&projects[i].PublicAccessLevel,
			&projects[i].Trashed,
			&projects[i].TagIds,
		)
		if err != nil {
			return errors.Tag(err, "scan projects")
		}
		projects[i].Owner.Id = projects[i].OwnerId
		projects[i].Owner.IdNoUnderscore = projects[i].OwnerId
		projects[i].LastUpdater.Id = projects[i].LastUpdatedBy
		projects[i].LastUpdater.IdNoUnderscore = projects[i].LastUpdatedBy
	}
	if err = r.Err(); err != nil {
		return errors.Tag(err, "iter projects cursor")
	}
	r.Close()
	d.Projects = projects

	var tagNames []string
	var tagIds sharedTypes.UUIDs

	var notificationIds sharedTypes.UUIDs
	var notificationKeys []string
	var notificationTemplateKeys []string
	var notificationMessageOptions []json.RawMessage

	err = br.QueryRow().Scan(
		&d.User.Email, &d.User.EmailConfirmedAt,
		&d.User.FirstName, &d.User.LastName,
		&tagIds, &tagNames,
		&notificationIds, &notificationKeys, &notificationTemplateKeys,
		&notificationMessageOptions,
	)
	if err != nil {
		return errors.Tag(err, "query user and tags")
	}
	d.User.IdNoUnderscore = userId

	d.Tags = make([]tag.Full, 0, len(tagIds))
	for i, id := range tagIds {
		t := tag.Full{
			Id:   id,
			Name: tagNames[i],
		}
		for _, p := range d.Projects {
			for _, tagId := range p.TagIds {
				if tagId == id {
					t.ProjectIds = append(t.ProjectIds, p.Id)
					break
				}
			}
		}
		if t.ProjectIds == nil {
			t.ProjectIds = make([]sharedTypes.UUID, 0)
		}
		d.Tags = append(d.Tags, t)
	}

	d.Notifications = make(notification.Notifications, 0, len(notificationIds))
	for i, id := range notificationIds {
		d.Notifications = append(d.Notifications, notification.Notification{
			Id:             id,
			Key:            notificationKeys[i],
			TemplateKey:    notificationTemplateKeys[i],
			MessageOptions: notificationMessageOptions[i],
		})
	}
	return nil
}
