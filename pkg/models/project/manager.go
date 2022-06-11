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

package project

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/edgedb/edgedb-go"
	"github.com/lib/pq"
	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/tag"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	spellingTypes "github.com/das7pad/overleaf-go/services/spelling/pkg/types"
)

type Manager interface {
	PrepareProjectCreation(ctx context.Context, p *ForCreation) error
	FinalizeProjectCreation(ctx context.Context, p *ForCreation) error
	SoftDelete(ctx context.Context, projectId []sharedTypes.UUID, userId sharedTypes.UUID, ipAddress string) error
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
	MoveFile(ctx context.Context, projectId, userId, folderId, fileId sharedTypes.UUID) (sharedTypes.Version, error)
	MoveFolder(ctx context.Context, projectId, userId, targetFolderId, folderId sharedTypes.UUID) (sharedTypes.Version, []Doc, error)
	RenameDoc(ctx context.Context, projectId, userId sharedTypes.UUID, d *Doc) (sharedTypes.Version, sharedTypes.PathName, error)
	RenameFile(ctx context.Context, projectId, userId sharedTypes.UUID, f *FileRef) (sharedTypes.Version, error)
	RenameFolder(ctx context.Context, projectId, userId sharedTypes.UUID, f *Folder) (sharedTypes.Version, []Doc, error)
	GetAuthorizationDetails(ctx context.Context, projectId, userId sharedTypes.UUID, accessToken AccessToken) (*AuthorizationDetails, error)
	GetForProjectJWT(ctx context.Context, projectId, userId sharedTypes.UUID, accessToken AccessToken) (*ForProjectJWT, int64, error)
	GetForZip(ctx context.Context, projectId sharedTypes.UUID, userId sharedTypes.UUID, accessToken AccessToken) (*ForZip, error)
	ValidateProjectJWTEpochs(ctx context.Context, projectId, userId sharedTypes.UUID, projectEpoch, userEpoch int64) error
	BumpLastOpened(ctx context.Context, projectId sharedTypes.UUID) error
	GetDoc(ctx context.Context, projectId, docId sharedTypes.UUID) (*Doc, error)
	GetFile(ctx context.Context, projectId, userId sharedTypes.UUID, accessToken AccessToken, fileId sharedTypes.UUID) (*FileWithParent, error)
	GetElementHintForOverwrite(ctx context.Context, projectId, userId, folderId sharedTypes.UUID, name sharedTypes.Filename) (sharedTypes.UUID, bool, error)
	GetElementByPath(ctx context.Context, projectId, userId sharedTypes.UUID, path sharedTypes.PathName) (sharedTypes.UUID, bool, error)
	GetJoinProjectDetails(ctx context.Context, projectId, userId sharedTypes.UUID, accessToken AccessToken) (*JoinProjectDetails, error)
	GetLoadEditorDetails(ctx context.Context, projectId, userId sharedTypes.UUID, accessToken AccessToken) (*LoadEditorDetails, error)
	GetProjectWithContent(ctx context.Context, projectId sharedTypes.UUID) ([]Doc, []FileRef, error)
	GetProject(ctx context.Context, projectId sharedTypes.UUID, target interface{}) error
	GetTokenAccessDetails(ctx context.Context, userId sharedTypes.UUID, privilegeLevel sharedTypes.PrivilegeLevel, accessToken AccessToken) (*ForTokenAccessDetails, error)
	GetTreeEntities(ctx context.Context, projectId, userId sharedTypes.UUID) ([]TreeEntity, error)
	GetProjectMembers(ctx context.Context, projectId sharedTypes.UUID) ([]user.AsProjectMember, error)
	GrantTokenAccess(ctx context.Context, projectId, userId sharedTypes.UUID, accessToken AccessToken, privilegeLevel sharedTypes.PrivilegeLevel) error
	GrantMemberAccess(ctx context.Context, projectId sharedTypes.UUID, epoch int64, userId sharedTypes.UUID, level sharedTypes.PrivilegeLevel) error
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
	RemoveMember(ctx context.Context, projectId []sharedTypes.UUID, actor, userId sharedTypes.UUID) error
	TransferOwnership(ctx context.Context, projectId, previousOwnerId, newOwnerId sharedTypes.UUID) (*user.WithPublicInfo, *user.WithPublicInfo, Name, error)
	CreateDoc(ctx context.Context, projectId, userId, folderId sharedTypes.UUID, d *Doc) (sharedTypes.Version, error)
	CreateFile(ctx context.Context, projectId, userId, folderId sharedTypes.UUID, f *FileRef) (sharedTypes.Version, error)
	ListProjects(ctx context.Context, userId sharedTypes.UUID) (List, error)
	GetProjectListDetails(ctx context.Context, userId sharedTypes.UUID, r *ForProjectList) error
}

func New(db *sql.DB) Manager {
	return &manager{db: db}
}

func rewriteEdgedbError(err error) error {
	if err == nil {
		return nil
	}
	// TODO: handle conflicting path -> edgedb.ConstraintViolationError
	if e, ok := err.(edgedb.Error); ok && e.Category(edgedb.NoDataError) {
		return &errors.NotFoundError{}
	}
	return err
}

func getErr(_ sql.Result, err error) error {
	return err
}

func rewritePostgresErr(err error) error {
	if err == nil {
		return nil
	}
	e, ok := err.(*pq.Error)
	if !ok {
		return err
	}
	if e.Constraint == "tree_nodes_pkey" {
		return ErrDuplicateNameInFolder
	}
	return err
}

type manager struct {
	db *sql.DB
	c  *edgedb.Client
}

func (m *manager) PrepareProjectCreation(ctx context.Context, p *ForCreation) error {
	ok := false
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if !ok {
			_ = tx.Rollback()
		}
	}()

	_, err = tx.ExecContext(
		ctx,
		`
WITH p AS (
    INSERT INTO projects
        (compiler, deleted_at, epoch, id, image_name, last_opened_at,
         last_updated_at, last_updated_by, name, owner_id, public_access_level,
         spell_check_language, tree_version)
        SELECT $3,
               $4,
               1,
               $5,
               $6,
               transaction_timestamp(),
               transaction_timestamp(),
               o.id,
               $7,
               o.id,
               'private',
               coalesce(
                       nullif($2, 'inherit'),
                       (o.editor_config ->> 'spellCheckLanguage')
                   ),
               1
        FROM users o
        WHERE id = $1)
INSERT
INTO project_members
(project_id, user_id, access_source, privilege_level, archived, trashed)
VALUES ($5, $1, 'owner', 'owner', FALSE, FALSE)
`,
		p.OwnerId, p.SpellCheckLanguage, p.Compiler, p.DeletedAt, p.Id,
		p.ImageName, p.Name,
	)
	if err != nil {
		return err
	}
	q, err := tx.PrepareContext(
		ctx,
		pq.CopyIn(
			"tree_nodes",
			"deleted_at", "id", "kind", "name", "parent_id", "path",
			"project_id",
		),
	)
	if err != nil {
		return errors.Tag(err, "prepare tree")
	}
	defer func() {
		if !ok && q != nil {
			_ = q.Close()
		}
	}()
	deletedAt := "1970-01-01"
	t := p.RootFolder
	_, err = q.ExecContext(
		ctx, deletedAt, t.Id, "folder", "", nil, "", p.Id,
	)
	if err != nil {
		return errors.Tag(err, "queue root folder")
	}
	err = t.WalkFolders(func(f *Folder, path sharedTypes.DirName) error {
		for _, d := range f.Docs {
			_, err = q.ExecContext(
				ctx,
				deletedAt, d.Id, "doc", d.Name, f.Id, path.Join(d.Name), p.Id,
			)
			if err != nil {
				return err
			}
		}
		for _, r := range f.FileRefs {
			_, err = q.ExecContext(
				ctx,
				deletedAt, r.Id, "file", r.Name, f.Id, path.Join(r.Name), p.Id,
			)
			if err != nil {
				return err
			}
		}
		for _, ff := range f.Folders {
			_, err = q.ExecContext(
				ctx,
				deletedAt, ff.Id, "folder", ff.Name, f.Id, ff.Path, p.Id,
			)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return errors.Tag(err, "queue tree")
	}
	if _, err = q.ExecContext(ctx); err != nil {
		return errors.Tag(err, "flush tree")
	}
	if err = q.Close(); err != nil {
		return errors.Tag(err, "close tree")
	}

	q, err = tx.PrepareContext(
		ctx,
		pq.CopyIn("docs", "id", "snapshot", "version"),
	)
	if err != nil {
		return errors.Tag(err, "prepare docs")
	}
	err = t.WalkDocs(func(e TreeElement, _ sharedTypes.PathName) error {
		d := e.(*Doc)
		_, err = q.ExecContext(ctx, d.Id, d.Snapshot, d.Version)
		return err
	})
	if err != nil {
		return errors.Tag(err, "queue docs")
	}
	if _, err = q.ExecContext(ctx); err != nil {
		return errors.Tag(err, "flush docs")
	}
	if err = q.Close(); err != nil {
		return err
	}

	q, err = tx.PrepareContext(
		ctx,
		pq.CopyIn(
			"files",
			"id", "created_at", "hash", "linked_file_data", "size",
		),
	)
	if err != nil {
		return errors.Tag(err, "prepare files")
	}
	err = t.WalkFiles(func(e TreeElement, _ sharedTypes.PathName) error {
		d := e.(*FileRef)
		_, err = q.ExecContext(
			ctx, d.Id, d.Created, d.Hash, d.LinkedFileData, d.Size,
		)
		return err
	})
	if err != nil {
		return errors.Tag(err, "queue files")
	}
	if _, err = q.ExecContext(ctx); err != nil {
		return errors.Tag(err, "flush files")
	}
	if err = q.Close(); err != nil {
		return errors.Tag(err, "close files")
	}

	ok = true
	if err = tx.Commit(); err != nil {
		return err
	}
	return err
}

func (m *manager) FinalizeProjectCreation(ctx context.Context, p *ForCreation) error {
	var rootDocId interface{} = nil
	if p.RootDoc.Id != (sharedTypes.UUID{}) {
		rootDocId = p.RootDoc.Id.String()
	}
	return getErr(m.db.ExecContext(ctx, `
UPDATE projects
SET deleted_at     = NULL,
    name           = $2,
    root_doc_id    = $3,
    root_folder_id = $4
WHERE id = $1
`, p.Id.String(), p.Name, rootDocId, p.RootFolder.Id.String()))
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
		err = m.db.QueryRowContext(ctx, `
UPDATE projects
SET token_ro        = COALESCE(token_ro, $3),
    token_rw        = COALESCE(token_rw, $4),
    token_rw_prefix = COALESCE(token_rw_prefix, $5)
WHERE id = $1
  AND owner_id = $2
  AND deleted_at IS NULL
RETURNING token_ro, token_rw
`,
			projectId, userId,
			tokens.ReadOnly, tokens.ReadAndWrite, tokens.ReadAndWritePrefix,
		).Scan(&persisted.ReadOnly, &persisted.ReadAndWrite)
		if err != nil {
			if e, ok := err.(*pq.Error); ok &&
				(e.Constraint == "projects_token_ro_key" ||
					e.Constraint == "projects_token_rw_prefix_key") {
				allErrors.Add(err)
				continue
			}
			return nil, err
		}
		if tokens.ReadOnly == persisted.ReadOnly {
			return &persisted, nil
		}
		// tokens are already populated
		return nil, nil
	}
	return nil, errors.Tag(allErrors, "bad random source")
}

func (m *manager) SetCompiler(ctx context.Context, projectId, userId sharedTypes.UUID, compiler sharedTypes.Compiler) error {
	return getErr(m.db.ExecContext(ctx, `
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
	return getErr(m.db.ExecContext(ctx, `
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
	return getErr(m.db.ExecContext(ctx, `
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
	return getErr(m.db.ExecContext(ctx, `
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
	return getErr(m.db.ExecContext(ctx, `
UPDATE projects
SET public_access_level = $3
WHERE id = $1
  AND owner_id = $2
  AND deleted_at IS NULL
  AND token_ro IS NOT NULL
`, projectId, userId, publicAccessLevel))
}

type transferOwnershipResult struct {
	ProjectExists bool                `edgedb:"project_exists"`
	AuthCheck     bool                `edgedb:"auth_check"`
	MemberCheck   bool                `edgedb:"member_check"`
	ProjectName   Name                `edgedb:"project_name"`
	NewOwner      user.WithPublicInfo `edgedb:"new_owner"`
	PreviousOwner user.WithPublicInfo `edgedb:"previous_owner"`
	AuditLogEntry bool                `edgedb:"audit_log_entry"`
}

func (m *manager) TransferOwnership(ctx context.Context, projectId, previousOwnerId, newOwnerId sharedTypes.UUID) (*user.WithPublicInfo, *user.WithPublicInfo, Name, error) {
	r := transferOwnershipResult{}
	err := m.c.QuerySingle(ctx, `
with
	previousOwner := (
		select User filter .id = <uuid>$1 and not exists .deleted_at
	),
	newOwner := (select User filter .id = <uuid>$2 and not exists .deleted_at),
	p := (select Project filter .id = <uuid>$0 and not exists .deleted_at),
	pWithAuth := (select p filter .owner = previousOwner),
	pWithMemberCheck := (
		select {
			(select pWithAuth filter newOwner in .access_ro),
			(select pWithAuth filter newOwner in .access_rw),
		}
		limit 1
	),
	pUpdated := (
		update pWithMemberCheck
		set {
			access_rw := distinct {
				previousOwner,
				(select pWithMemberCheck.access_rw filter .id != <uuid>$2)
			},
			access_ro -= newOwner,
			access_token_ro -= newOwner,
			access_token_rw -= newOwner,
			epoch := pWithMemberCheck.epoch + 1,
			owner := newOwner,
		}
	),
	auditLogEntry := (
		insert ProjectAuditLogEntry {
			project := pUpdated,
			initiator := previousOwner,
			operation := 'transfer-ownership',
			info := <json>{
				newOwnerId := newOwner.id,
				previousOwnerId := previousOwner.id,
			}
		}
	)
select {
	project_exists := exists p,
	auth_check := exists pWithAuth,
	member_check := exists pWithMemberCheck,
	project_name := pUpdated.name ?? "",
	new_owner := newOwner {
		email: { email }, id, first_name, last_name,
	},
	previous_owner := previousOwner {
		email: { email }, id, first_name, last_name,
	},
	audit_log_entry := exists auditLogEntry,
}
`, &r, projectId, previousOwnerId, newOwnerId)
	if err != nil {
		return nil, nil, "", rewriteEdgedbError(err)
	}
	switch {
	case !r.ProjectExists:
		return nil, nil, "", &errors.NotFoundError{}
	case !r.AuthCheck:
		return nil, nil, "", &errors.NotAuthorizedError{}
	case !r.MemberCheck:
		return nil, nil, "", &errors.InvalidStateError{
			Msg: "new owner is not an invited user",
		}
	}
	previousOwner := &r.PreviousOwner
	newOwner := &r.NewOwner
	return previousOwner, newOwner, r.ProjectName, nil
}

func (m *manager) Rename(ctx context.Context, projectId, userId sharedTypes.UUID, name Name) error {
	return getErr(m.db.ExecContext(ctx, `
UPDATE projects
SET name = $3
WHERE id = $1
  AND deleted_at IS NULL
  AND owner_id = $2
`, projectId, userId, name))
}

func (m *manager) AddFolder(ctx context.Context, projectId, userId, parent sharedTypes.UUID, f *Folder) (sharedTypes.Version, error) {
	var treeVersion sharedTypes.Version
	return treeVersion, m.db.QueryRowContext(ctx, `
WITH f AS (
    INSERT INTO tree_nodes
        (deleted_at, id, kind, name, parent_id, path, project_id)
        SELECT '1970-01-01',
               $4,
               'folder',
               $5,
               $3,
               CONCAT(t.path, $5::TEXT, '/'),
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

func (m *manager) deleteTreeLeaf(ctx context.Context, projectId, userId, nodeId sharedTypes.UUID, kind string) (sharedTypes.Version, error) {
	var treeVersion sharedTypes.Version
	return treeVersion, m.db.QueryRowContext(ctx, `
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
	return m.deleteTreeLeaf(ctx, projectId, userId, docId, "doc")
}

func (m *manager) DeleteFile(ctx context.Context, projectId, userId, fileId sharedTypes.UUID) (sharedTypes.Version, error) {
	return m.deleteTreeLeaf(ctx, projectId, userId, fileId, "file")
}

func (m *manager) DeleteFolder(ctx context.Context, projectId, userId, folderId sharedTypes.UUID) (sharedTypes.Version, error) {
	var v sharedTypes.Version
	return v, rewritePostgresErr(m.db.QueryRowContext(ctx, `
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

func (m *manager) moveTreeLeaf(ctx context.Context, projectId, userId, folderId, nodeId sharedTypes.UUID, kind string) (sharedTypes.Version, sharedTypes.PathName, error) {
	var treeVersion sharedTypes.Version
	var path sharedTypes.PathName
	return treeVersion, path, m.db.QueryRowContext(ctx, `
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
                 path = CONCAT(f.path, SPLIT_PART(t.path, '/', -1))
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
	return m.moveTreeLeaf(ctx, projectId, userId, folderId, docId, "doc")
}

func (m *manager) MoveFile(ctx context.Context, projectId, userId, folderId, fileId sharedTypes.UUID) (sharedTypes.Version, error) {
	v, _, err := m.moveTreeLeaf(ctx, projectId, userId, folderId, fileId, "file")
	return v, err
}

func (m *manager) MoveFolder(ctx context.Context, projectId, userId, targetFolderId, folderId sharedTypes.UUID) (sharedTypes.Version, []Doc, error) {
	var v sharedTypes.Version
	var docIds []sharedTypes.UUID
	var docPaths []string
	err := m.db.QueryRowContext(ctx, `
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
             WHERE t.project_id = node.project_id AND
                   t.deleted_at = '1970-01-01' AND
                   starts_with(t.path, node.path)
             RETURNING t.id, t.kind, t.path),
     updated_docs AS (SELECT array_agg(id) as ids, array_agg(path) as paths
                      FROM updated_children
                      WHERE kind = 'doc'),
     updated_version AS (
         UPDATE projects p
             SET last_updated_by = $2,
                 last_updated_at = transaction_timestamp(),
                 tree_version = tree_version + 1
             FROM updated
             WHERE p.id = $1
             RETURNING p.tree_version)

SELECT updated_version.tree_version, updated_docs.ids, updated_docs.paths
FROM updated_version,
     updated_docs
`, projectId, userId, folderId, targetFolderId).
		Scan(&v, pq.Array(&docIds), pq.Array(&docPaths))
	if err != nil {
		return 0, nil, rewritePostgresErr(err)
	}
	var docs []Doc
	for i, id := range docIds {
		d := Doc{}
		d.Id = id
		d.ResolvedPath = sharedTypes.PathName(docPaths[i])
		docs = append(docs, d)
	}
	return v, docs, nil
}

func (m *manager) renameTreeLeaf(ctx context.Context, projectId, userId, nodeId sharedTypes.UUID, kind string, name sharedTypes.Filename) (sharedTypes.Version, sharedTypes.PathName, error) {
	var treeVersion sharedTypes.Version
	var path sharedTypes.PathName
	return treeVersion, path, m.db.QueryRowContext(ctx, `
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
             SET path = CONCAT(node.parent_path, $5::TEXT)
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
	return m.renameTreeLeaf(ctx, projectId, userId, d.Id, "doc", d.Name)
}

func (m *manager) RenameFile(ctx context.Context, projectId, userId sharedTypes.UUID, f *FileRef) (sharedTypes.Version, error) {
	v, _, err := m.renameTreeLeaf(ctx, projectId, userId, f.Id, "file", f.Name)
	return v, err
}

func (m *manager) RenameFolder(ctx context.Context, projectId, userId sharedTypes.UUID, f *Folder) (sharedTypes.Version, []Doc, error) {
	var v sharedTypes.Version
	var docIds []sharedTypes.UUID
	var docPaths []string
	err := m.db.QueryRowContext(ctx, `
WITH node AS (SELECT t.id,
                     t.project_id,
                     t.path,
                     char_length(t.path) + 1     AS old_end,
                     parent.path AS parent_path
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
     updated AS (
         UPDATE tree_nodes t
             SET name = $4,
                 path = concat(node.parent_path, $4, '/')
             FROM node
             WHERE t.id = node.id
             RETURNING t.path),
     updated_children AS (
         UPDATE tree_nodes t
             SET path = concat(updated.path, substr(t.path, node.old_end))
             FROM node, updated
             WHERE t.project_id = node.project_id AND
                   t.deleted_at = '1970-01-01' AND
                   starts_with(t.path, node.path)
             RETURNING t.id, t.kind, t.path),
     updated_docs AS (SELECT array_agg(id) as ids, array_agg(path) as paths
                      FROM updated_children
                      WHERE kind = 'doc'),
     updated_version AS (
         UPDATE projects p
             SET last_updated_by = $2,
                 last_updated_at = transaction_timestamp(),
                 tree_version = tree_version + 1
             FROM updated
             WHERE p.id = $1
             RETURNING p.tree_version)

SELECT updated_version.tree_version, updated_docs.ids, updated_docs.paths
FROM updated_version,
     updated_docs
`, projectId, userId, f.Id, f.Name).
		Scan(&v, pq.Array(&docIds), pq.Array(&docPaths))
	if err != nil {
		return 0, nil, rewritePostgresErr(err)
	}
	var docs []Doc
	for i, id := range docIds {
		d := Doc{}
		d.Id = id
		d.ResolvedPath = sharedTypes.PathName(docPaths[i])
		docs = append(docs, d)
	}
	return v, docs, nil
}

func (m *manager) ArchiveForUser(ctx context.Context, projectId, userId sharedTypes.UUID) error {
	return getErr(m.db.ExecContext(ctx, `
UPDATE project_members
SET archived = TRUE,
    trashed  = FALSE
WHERE project_id = $1
  AND user_id = $2
`, projectId, userId))
}

func (m *manager) UnArchiveForUser(ctx context.Context, projectId, userId sharedTypes.UUID) error {
	return getErr(m.db.ExecContext(ctx, `
UPDATE project_members
SET archived = FALSE
WHERE project_id = $1
  AND user_id = $2
`, projectId, userId))
}

func (m *manager) TrashForUser(ctx context.Context, projectId, userId sharedTypes.UUID) error {
	return getErr(m.db.ExecContext(ctx, `
UPDATE project_members
SET archived = FALSE,
    trashed  = TRUE
WHERE project_id = $1
  AND user_id = $2
`, projectId, userId))
}

func (m *manager) UnTrashForUser(ctx context.Context, projectId, userId sharedTypes.UUID) error {
	return getErr(m.db.ExecContext(ctx, `
UPDATE project_members
SET trashed = FALSE
WHERE project_id = $1
  AND user_id = $2
`, projectId, userId))
}

var ErrEpochIsNotStable = errors.New("epoch is not stable")

func (m *manager) GetProjectNames(ctx context.Context, userId sharedTypes.UUID) (Names, error) {
	var raw []string
	err := m.db.QueryRowContext(ctx, `
SELECT array_agg(name)
FROM projects p
	INNER JOIN project_members pm ON p.id = pm.project_id
WHERE user_id = $1
`, userId).Scan(pq.Array(&raw))
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
	err := m.db.QueryRowContext(ctx, `
SELECT coalesce(pm.access_source::TEXT, ''),
       coalesce(pm.privilege_level::TEXT, ''),
       p.epoch,
       p.public_access_level,
       COALESCE(p.token_ro, ''),
       COALESCE(p.token_rw, '')
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
		return nil, err
	}
	return p.GetPrivilegeLevel(userId, accessToken)
}

func (m *manager) GetForProjectJWT(ctx context.Context, projectId, userId sharedTypes.UUID, accessToken AccessToken) (*ForProjectJWT, int64, error) {
	p := ForProjectJWT{}
	var userEpoch int64
	err := m.db.QueryRowContext(ctx, `
SELECT coalesce(pm.access_source::TEXT, ''),
       coalesce(pm.privilege_level::TEXT, ''),
       p.epoch,
       p.public_access_level,
       COALESCE(p.token_ro, ''),
       COALESCE(p.token_rw, ''),
       o.features,
       COALESCE(u.epoch, 0)
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
	if userId == (sharedTypes.UUID{}) {
		err = m.db.QueryRowContext(ctx, `
SELECT TRUE
FROM projects
WHERE id = $1 AND epoch = $2
`, projectId, projectEpoch).Scan(&ok)
	} else {
		err = m.db.QueryRowContext(ctx, `
SELECT TRUE
FROM projects p, users u
WHERE p.id = $1 AND p.epoch = $2 AND u.id = $3 AND u.epoch = $4
`, projectId, projectEpoch, userId, userEpoch).Scan(&ok)
	}
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if err == nil && ok {
		return nil
	}
	return &errors.UnauthorizedError{Reason: "epoch mismatch"}
}

func (m *manager) GetDoc(ctx context.Context, projectId, docId sharedTypes.UUID) (*Doc, error) {
	d := Doc{}
	err := m.db.QueryRowContext(ctx, `
SELECT t.path, d.snapshot, d.version
FROM docs d
         INNER JOIN tree_nodes t ON d.id = t.id
         INNER JOIN projects p ON t.project_id = p.id
WHERE d.id = $2
  AND t.project_id = $1
  AND t.deleted_at = '1970-01-01'
  AND p.deleted_at IS NULL
`, projectId, docId).Scan(
		&d.ResolvedPath,
		&d.Snapshot,
		&d.Version,
	)
	if err == sql.ErrNoRows {
		return nil, &errors.ErrorDocNotFound{}
	}
	d.Id = docId
	d.Name = d.ResolvedPath.Filename()
	return &d, err
}

func (m *manager) RestoreDoc(ctx context.Context, projectId, userId, docId sharedTypes.UUID, name sharedTypes.Filename) (sharedTypes.Version, sharedTypes.UUID, error) {
	var v sharedTypes.Version
	var rootFolderId sharedTypes.UUID
	return v, rootFolderId, rewritePostgresErr(m.db.QueryRowContext(ctx, `
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
	d := LinkedFileData{}
	err := m.db.QueryRowContext(ctx, `
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
		&f.ResolvedPath, &f.ParentId, &d, &f.Size,
	)
	f.Id = fileId
	f.Name = f.ResolvedPath.Filename()
	if d.Provider != "" {
		f.LinkedFileData = &d
	}
	return &f, err
}

func (m *manager) GetElementHintForOverwrite(ctx context.Context, projectId, userId, folderId sharedTypes.UUID, name sharedTypes.Filename) (sharedTypes.UUID, bool, error) {
	var nodeId sharedTypes.UUID
	var kind string
	err := m.db.QueryRowContext(ctx, `
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
  AND (t.path = $4 OR t.path LIKE CONCAT('%/', $4::TEXT))
`, projectId, userId, folderId, name).Scan(&nodeId, &kind)
	if err == sql.ErrNoRows {
		return nodeId, false, nil
	}
	if kind == "folder" {
		return nodeId, false, &errors.UnprocessableEntityError{
			Msg: "element is a folder",
		}
	}
	return nodeId, kind == "doc", err
}

func (m *manager) GetElementByPath(ctx context.Context, projectId, userId sharedTypes.UUID, path sharedTypes.PathName) (sharedTypes.UUID, bool, error) {
	var id sharedTypes.UUID
	var isDoc bool
	return id, isDoc, m.db.QueryRowContext(ctx, `
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
	r, err := m.db.QueryContext(ctx, `
SELECT t.id, t.path, COALESCE(d.snapshot, ''), COALESCE(d.version, -1)
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
	defer func() { _ = r.Close() }()
	nodes := make([]Doc, 0)
	for i := 0; r.Next(); i++ {
		nodes = append(nodes, Doc{})
		err = r.Scan(
			&nodes[i].Id, &nodes[i].ResolvedPath, &nodes[i].Snapshot,
			&nodes[i].Version,
		)
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
	return &p, m.db.QueryRowContext(ctx, `
WITH tree AS
         (SELECT t.project_id,
                 array_agg(t.id)                     as ids,
                 array_agg(t.kind)                   as kinds,
                 array_agg(t.path)                   as paths,
                 array_agg(COALESCE(d.snapshot, '')) as snapshots
          FROM tree_nodes t
                   LEFT JOIN docs d on t.id = d.id
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
		pq.Array(&p.treeIds),
		pq.Array(&p.treeKinds),
		pq.Array(&p.treePaths),
		pq.Array(&p.docSnapshots),
	)
}

func (m *manager) GetJoinProjectDetails(ctx context.Context, projectId, userId sharedTypes.UUID, accessToken AccessToken) (*JoinProjectDetails, error) {
	d := &JoinProjectDetails{}
	d.Project.Id = projectId
	d.Project.RootFolder = NewFolder("")
	d.Project.DeletedDocs = make([]CommonTreeFields, 0)

	var deletedDocIds []sharedTypes.UUID
	var deletedDocNames []string

	// TODO: fetch file details `created_at` and `linked_file_data`
	// TODO: let frontend query members/invites on modal open (again)
	err := m.db.QueryRowContext(ctx, `
WITH tree AS
         (SELECT t.project_id,
                 array_agg(t.id)   as ids,
                 array_agg(t.kind) as kinds,
                 array_agg(t.path) as paths
          FROM tree_nodes t
          WHERE t.project_id = $1
            AND t.deleted_at = '1970-01-01'
            AND t.parent_id IS NOT NULL
          GROUP BY t.project_id),
     deleted_docs AS (SELECT t.project_id,
                             array_agg(t.id)   as ids,
                             array_agg(t.name) as names
                      FROM tree_nodes t
                      WHERE t.project_id = $1
                        AND t.deleted_at != '1970-01-01'
                      GROUP BY t.project_id)

SELECT coalesce(pm.access_source::TEXT, ''),
       coalesce(pm.privilege_level::TEXT, ''),
       p.compiler,
       p.epoch,
       p.image_name,
       p.name,
       p.owner_id,
       p.public_access_level,
       COALESCE(p.root_doc_id, '00000000-0000-0000-0000-000000000000'::UUID),
       p.root_folder_id,
       p.spell_check_language,
       COALESCE(p.token_ro, ''),
       COALESCE(p.token_rw, ''),
       p.tree_version,
       o.features,
       o.email,
       o.first_name,
       o.last_name,
       tree.ids,
       tree.kinds,
       tree.paths,
       deleted_docs.ids,
       deleted_docs.names
FROM projects p
         INNER JOIN users o ON p.owner_id = o.id
         LEFT JOIN tree ON (p.id = tree.project_id)
         LEFT JOIN deleted_docs ON (p.id = deleted_docs.project_id)
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
		&d.Project.Member.AccessSource,
		&d.Project.Member.PrivilegeLevel,
		&d.Project.Compiler,
		&d.Project.Epoch,
		&d.Project.ImageName,
		&d.Project.Name,
		&d.Project.OwnerId,
		&d.Project.PublicAccessLevel,
		&d.Project.RootDoc.Id,
		&d.Project.RootFolder.Id,
		&d.Project.SpellCheckLanguage,
		&d.Project.Tokens.ReadOnly,
		&d.Project.Tokens.ReadAndWrite,
		&d.Project.Version,
		&d.Project.OwnerFeatures,
		&d.Owner.Email,
		&d.Owner.FirstName,
		&d.Owner.LastName,
		pq.Array(&d.Project.treeIds),
		pq.Array(&d.Project.treeKinds),
		pq.Array(&d.Project.treePaths),
		pq.Array(&deletedDocIds),
		pq.Array(&deletedDocNames),
	)
	if err != nil {
		return nil, err
	}
	for i, id := range deletedDocIds {
		d.Project.DeletedDocs = append(d.Project.DeletedDocs, CommonTreeFields{
			Id:   id,
			Name: sharedTypes.Filename(deletedDocNames[i]),
		})
	}
	d.Owner.Id = d.Project.OwnerId
	return d, nil
}

func (m *manager) BumpLastOpened(ctx context.Context, projectId sharedTypes.UUID) error {
	return getErr(m.db.ExecContext(ctx, `
UPDATE projects
SET last_opened_at = transaction_timestamp()
WHERE id = $1
`, projectId))
}

func (m *manager) GetLoadEditorDetails(ctx context.Context, projectId, userId sharedTypes.UUID, accessToken AccessToken) (*LoadEditorDetails, error) {
	d := LoadEditorDetails{}
	err := m.db.QueryRowContext(ctx, `
SELECT coalesce(pm.access_source::TEXT, ''),
       coalesce(pm.privilege_level::TEXT, ''),
       p.compiler,
       p.epoch,
       p.image_name,
       p.name,
       p.public_access_level,
       COALESCE(p.token_ro, ''),
       COALESCE(p.token_rw, ''),
       p.tree_version,
       COALESCE(d.id, '00000000-0000-0000-0000-000000000000'::UUID),
       COALESCE(d.path, ''),
       o.features,
       coalesce(u.editor_config, '{}'),
       coalesce(u.email, ''),
       coalesce(u.epoch, 0),
       coalesce(u.first_name, ''),
       coalesce(u.last_name, '')
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
		&d.Project.RootDoc.ResolvedPath,
		&d.Project.OwnerFeatures,
		&d.User.EditorConfig,
		&d.User.Email,
		&d.User.Epoch,
		&d.User.FirstName,
		&d.User.LastName,
	)
	if err != nil {
		return nil, err
	}
	d.Project.Id = projectId
	d.User.Id = userId
	return &d, err
}

func (m *manager) GetProject(ctx context.Context, projectId sharedTypes.UUID, target interface{}) error {
	var q string
	switch p := target.(type) {
	case *LastUpdatedAtField:
		return m.db.QueryRowContext(ctx, `
SELECT last_updated_at
FROM projects
WHERE id = $1 AND deleted_at IS NULL
`, projectId).Scan(&p.LastUpdatedAt)
	case *ForProjectInvite:
		q = `
select Project {
	access_ro,
	access_rw,
	access_token_ro,
	access_token_rw,
	epoch,
	id,
	name,
	owner,
	public_access_level,
}
filter .id = <uuid>$0 and not exists .deleted_at
`
	case *ForClone:
		q = `
select Project {
	access_ro,
	access_rw,
	access_token_ro,
	access_token_rw,
	compiler,
	docs: {
		name,
		resolved_path,
		snapshot,
	},
	files: {
		hash,
		id,
		linked_file_data: {
			provider,
			source_project_id,
			source_entity_path,
			source_output_file_path,
			url,
		},
		name,
		resolved_path,
		size,
	},
	image_name,
	owner,
	public_access_level,
	root_doc: { resolved_path },
	spell_check_language,
}
filter .id = <uuid>$0 and not exists .deleted_at
`
	default:
		return errors.New("missing query for target")
	}
	if err := m.c.QuerySingle(ctx, q, target, projectId); err != nil {
		return rewriteEdgedbError(err)
	}
	return nil
}

type TreeEntity struct {
	Path string `json:"path"`
	Type string `json:"type"`
}

func (m *manager) GetTreeEntities(ctx context.Context, projectId, userId sharedTypes.UUID) ([]TreeEntity, error) {
	r, err := m.db.QueryContext(ctx, `
SELECT path, kind
FROM tree_nodes t
INNER JOIN projects p ON t.project_id = p.id
INNER JOIN project_members pm ON (p.id = pm.project_id AND
                                  pm.user_id = $2)
WHERE t.project_id = $1
  AND p.deleted_at IS NULL
AND (t.kind = 'doc' OR t.kind = 'file')
`, projectId, userId)
	if err != nil {
		return nil, err
	}
	defer func() { _ = r.Close() }()
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
	r, err := m.db.QueryContext(ctx, `
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
	defer func() { _ = r.Close() }()
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

func (m *manager) GrantMemberAccess(ctx context.Context, projectId sharedTypes.UUID, epoch int64, userId sharedTypes.UUID, level sharedTypes.PrivilegeLevel) error {
	var q string
	switch level {
	case sharedTypes.PrivilegeLevelReadAndWrite:
		q = `
with
	u := (select User filter .id = <uuid>$0 and not exists .deleted_at)
update Project
filter
	.id = <uuid>$1
and .epoch = <int64>$2
and not exists .deleted_at
set {
	epoch := Project.epoch + 1,
	access_rw := distinct (Project.access_rw union {u}),
	access_ro -= u,
}
`
	case sharedTypes.PrivilegeLevelReadOnly:
		q = `
with
	u := (select User filter .id = <uuid>$0 and not exists .deleted_at)
update Project
filter
	.id = <uuid>$1
and .epoch = <int64>$2
and not exists .deleted_at
set {
	epoch := Project.epoch + 1,
	access_ro := distinct (Project.access_ro union {u}),
	access_rw -= u,
}
`
	default:
		return errors.New("invalid member access level: " + string(level))
	}

	err := m.c.QuerySingle(ctx, q, &IdField{}, userId, projectId, epoch)
	if err != nil {
		err = rewriteEdgedbError(err)
		if errors.IsNotFoundError(err) {
			return ErrEpochIsNotStable
		}
		return err
	}
	return nil
}

func (m *manager) GetTokenAccessDetails(ctx context.Context, userId sharedTypes.UUID, privilegeLevel sharedTypes.PrivilegeLevel, accessToken AccessToken) (*ForTokenAccessDetails, error) {
	p := ForTokenAccessDetails{}
	q, err := accessToken.toQueryParameters(privilegeLevel)
	if err != nil {
		return nil, err
	}
	err = m.db.QueryRowContext(ctx, `
SELECT coalesce(pm.access_source::TEXT, ''),
       coalesce(pm.privilege_level::TEXT, ''),
       p.id,
       p.epoch,
       COALESCE(p.token_ro, ''),
       COALESCE(p.token_rw, '')
FROM projects p
         LEFT JOIN project_members pm ON (p.id = pm.project_id AND
                                          pm.user_id = $1)
WHERE (p.token_ro = $2 OR p.token_rw_prefix = $3)
  AND p.public_access_level = 'tokenBased'
  AND p.deleted_at IS NULL
`, userId, q.tokenRO, q.tokenRWPrefix).Scan(
		&p.Member.AccessSource, &p.Member.PrivilegeLevel,
		&p.Id, &p.Epoch, &p.Tokens.ReadOnly, &p.Tokens.ReadAndWrite,
	)
	if err != nil {
		return nil, err
	}
	p.PublicAccessLevel = TokenBasedAccess
	return &p, nil
}

func (m *manager) GrantTokenAccess(ctx context.Context, projectId, userId sharedTypes.UUID, accessToken AccessToken, privilegeLevel sharedTypes.PrivilegeLevel) error {
	q, err := accessToken.toQueryParameters(privilegeLevel)
	if err != nil {
		return err
	}
	return getErr(m.db.ExecContext(ctx, `
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

func (m *manager) RemoveMember(ctx context.Context, projectIds []sharedTypes.UUID, actor, userId sharedTypes.UUID) error {
	return getErr(m.db.ExecContext(ctx, `
WITH pm AS (
    DELETE FROM project_members pm
        USING projects p
        WHERE pm.project_id = ANY ($1)
            AND pm.user_id = $3
            AND p.owner_id != $3
            AND (p.owner_id = $2 OR $2 = $3)
        RETURNING project_id)
UPDATE projects
SET epoch = epoch + 1
WHERE id = pm.project_id
`, pq.Array(projectIds), actor, userId))
}

func (m *manager) SoftDelete(ctx context.Context, projectIds []sharedTypes.UUID, userId sharedTypes.UUID, ipAddress string) error {
	blob, err := json.Marshal(map[string]string{
		"ipAddress": ipAddress,
	})
	if err != nil {
		return err
	}
	r, err := m.db.ExecContext(ctx, `
WITH soft_deleted AS (
    UPDATE projects
        SET deleted_at = transaction_timestamp(),
            epoch = epoch + 1
        WHERE id = ANY ($1) AND owner_id = $2 AND deleted_at IS NULL
        RETURNING id)

INSERT
INTO project_audit_log
(id, info, initiator_id, operation, project_id, timestamp)
SELECT gen_random_uuid(),
       $3,
       $2,
       'soft-deletion',
       id,
       transaction_timestamp()
FROM soft_deleted
`, pq.Array(projectIds), userId, string(blob))
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != int64(len(projectIds)) {
		return errors.New("incomplete soft deletion")
	}
	return nil
}

func (m *manager) HardDelete(ctx context.Context, projectId sharedTypes.UUID) error {
	r, err := m.db.ExecContext(ctx, `
DELETE
FROM projects
WHERE id = $1
  AND deleted_at IS NOT NULL
`, projectId.String())
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return &errors.UnprocessableEntityError{
			Msg: "user missing or not deleted",
		}
	}
	return nil
}

func (m *manager) ProcessSoftDeleted(ctx context.Context, cutOff time.Time, fn func(projectId sharedTypes.UUID) bool) error {
	ids := make([]sharedTypes.UUID, 0, 100)
	for {
		ids = ids[:0]
		r := m.db.QueryRowContext(ctx, `
WITH ids AS (SELECT id
             FROM projects
             WHERE deleted_at <= $1
             ORDER BY deleted_at
             LIMIT 100)
SELECT array_agg(ids)
FROM ids
`, cutOff)
		if err := r.Scan(pq.Array(&ids)); err != nil {
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
	return name, m.db.QueryRowContext(ctx, `
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
	return getErr(m.db.ExecContext(ctx, `
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
	var v sharedTypes.Version
	return v, rewritePostgresErr(m.db.QueryRowContext(ctx, `
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
             (deleted_at, id, kind, name, parent_id, path, project_id)
             SELECT '1970-01-01',
                    $4,
                    'doc',
                    $5,
                    f.id,
                    CONCAT(f.path, $5::TEXT),
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

func (m *manager) CreateFile(ctx context.Context, projectId, userId, folderId sharedTypes.UUID, f *FileRef) (sharedTypes.Version, error) {
	var v sharedTypes.Version
	return v, rewritePostgresErr(m.db.QueryRowContext(ctx, `
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
             (deleted_at, id, kind, name, parent_id, path, project_id)
             SELECT '1970-01-01',
                    $4,
                    'file',
                    $5,
                    f.id,
                    CONCAT(f.path, $5::TEXT),
                    $1
             FROM f
             RETURNING id),
     inserted_file AS (
         INSERT INTO files
             (id, created_at, hash, linked_file_data, size)
             SELECT inserted_tree_node.id, transaction_timestamp(), $6, $7, $8
             FROM inserted_tree_node
			 RETURNING FALSE)

UPDATE projects p
SET last_updated_by = $2,
    last_updated_at = transaction_timestamp(),
    tree_version    = tree_version + 1
FROM inserted_file
WHERE p.id = $1
RETURNING p.tree_version
`,
		projectId, userId, folderId,
		f.Id, f.Name, f.Hash, f.LinkedFileData, f.Size,
	).Scan(&v))
}

type ForProjectList struct {
	User          user.ProjectListViewCaller
	Tags          tag.Tags
	Projects      List
	Collaborators user.BulkFetched
}

func (m *manager) ListProjects(ctx context.Context, userId sharedTypes.UUID) (List, error) {
	r, err := m.db.QueryContext(ctx, `
SELECT access_source,
       archived,
       epoch,
       id,
       last_updated_at,
       COALESCE(last_updated_by, '00000000-0000-0000-0000-000000000000'::UUID),
       name,
       owner_id,
       privilege_level,
       public_access_level,
       trashed
FROM projects p
         INNER JOIN project_members pm ON p.id = pm.project_id
WHERE pm.user_id = $1
  AND p.deleted_at IS NULL;
`, userId)
	if err != nil {
		return nil, err
	}
	defer func() { _ = r.Close() }()
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
			&projects[i].Name,
			&projects[i].OwnerId,
			&projects[i].PrivilegeLevel,
			&projects[i].PublicAccessLevel,
			&projects[i].Trashed,
		)
		if err != nil {
			return nil, err
		}
	}
	return projects, r.Err()
}

func (m *manager) GetProjectListDetails(ctx context.Context, userId sharedTypes.UUID, d *ForProjectList) error {
	// TODO: can we query in parallel from a tx? how many RTTs?
	eg, pCtx := errgroup.WithContext(ctx)

	// User
	eg.Go(func() error {
		return m.db.QueryRowContext(pCtx, `
SELECT id, email, email_confirmed_at, first_name, last_name
FROM users
WHERE id = $1
  AND deleted_at IS NULL;
`, userId).Scan(
			&d.User.Id, &d.User.Email, &d.User.EmailConfirmedAt,
			&d.User.FirstName, &d.User.LastName)
	})

	// Tags
	eg.Go(func() error {
		r, err := m.db.QueryContext(pCtx, `
SELECT id, name, array_remove(array_agg(project_id), NULL)
FROM tags t
         LEFT JOIN tag_entries te ON t.id = te.tag_id
WHERE t.user_id = $1
GROUP BY t.id;
`, userId)
		if err != nil {
			return err
		}
		defer func() { _ = r.Close() }()

		for i := 0; r.Next(); i++ {
			d.Tags = append(d.Tags, tag.Full{})
			err = r.Scan(
				&d.Tags[i].Id, &d.Tags[i].Name,
				pq.Array(&d.Tags[i].ProjectIds),
			)
			if err != nil {
				return err
			}
		}
		return r.Err()
	})

	// Collaborators
	eg.Go(func() error {
		r, err := m.db.QueryContext(pCtx, `
WITH p AS (SELECT owner_id, last_updated_by
           FROM projects p
                    INNER JOIN project_members pm ON p.id = pm.project_id
           WHERE pm.user_id = $1)
SELECT u.id, email, first_name, last_name
FROM users u
         INNER JOIN p ON (u.id = p.owner_id OR u.id = p.last_updated_by)
WHERE u.deleted_at IS NULL;
`, userId)
		if err != nil {
			return err
		}
		defer func() { _ = r.Close() }()
		if err = d.Collaborators.ScanInto(r); err != nil {
			return err
		}
		return r.Err()
	})

	// Projects
	eg.Go(func() error {
		var err error
		d.Projects, err = m.ListProjects(ctx, userId)
		if err != nil {
			return errors.Tag(err, "list projects")
		}
		return nil
	})
	if err := eg.Wait(); err != nil {
		return err
	}
	// The projects and collaborators queries are racing.
	// Check for missing users and back-fill them.
	fetched := make(map[sharedTypes.UUID]struct{}, len(d.Collaborators)+1)
	fetched[sharedTypes.UUID{}] = struct{}{}
	for _, u := range d.Collaborators {
		fetched[u.Id] = struct{}{}
	}
	var missing []sharedTypes.UUID
	for _, p := range d.Projects {
		if _, got := fetched[p.OwnerId]; !got {
			missing = append(missing, p.OwnerId)
		}
		if _, got := fetched[p.LastUpdatedBy]; !got {
			missing = append(missing, p.LastUpdatedBy)
		}
	}
	if len(missing) == 0 {
		return nil
	}

	r, err := m.db.QueryContext(ctx, `
SELECT u.id, email, first_name, last_name
FROM users u
WHERE id = ANY ($1)
  AND u.deleted_at IS NULL;
`, pq.Array(missing))
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()
	if err = d.Collaborators.ScanInto(r); err != nil {
		return err
	}
	return r.Err()
}
