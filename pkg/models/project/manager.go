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
	"strconv"
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
	SoftDelete(ctx context.Context, projectId, userId sharedTypes.UUID, ipAddress string) error
	HardDelete(ctx context.Context, projectId sharedTypes.UUID) error
	ProcessSoftDeleted(ctx context.Context, cutOff time.Time, fn func(projectId sharedTypes.UUID) bool) error
	GetDeletedProjectsName(ctx context.Context, projectId, userId sharedTypes.UUID) (Name, error)
	Restore(ctx context.Context, projectId, userId sharedTypes.UUID, name Name) error
	AddFolder(ctx context.Context, projectId, userId, parent sharedTypes.UUID, f *Folder) (sharedTypes.Version, error)
	DeleteDoc(ctx context.Context, projectId, userId, docId sharedTypes.UUID) (sharedTypes.Version, error)
	DeleteFile(ctx context.Context, projectId, userId, fileId sharedTypes.UUID) (sharedTypes.Version, error)
	DeleteFolder(ctx context.Context, projectId, userId, folderId sharedTypes.UUID) (sharedTypes.Version, error)
	RestoreDoc(ctx context.Context, projectId, userId, docId sharedTypes.UUID, name sharedTypes.Filename) (*DocWithParent, sharedTypes.Version, error)
	MoveDoc(ctx context.Context, projectId, userId, folderId, docId sharedTypes.UUID) (sharedTypes.Version, sharedTypes.PathName, error)
	MoveFile(ctx context.Context, projectId, userId, folderId, fileId sharedTypes.UUID) (sharedTypes.Version, error)
	MoveFolder(ctx context.Context, projectId, userId, targetFolderId, folderId sharedTypes.UUID) (sharedTypes.Version, []Doc, error)
	RenameDoc(ctx context.Context, projectId, userId sharedTypes.UUID, d *Doc) (sharedTypes.Version, sharedTypes.DirName, error)
	RenameFile(ctx context.Context, projectId, userId sharedTypes.UUID, f *FileRef) (sharedTypes.Version, error)
	RenameFolder(ctx context.Context, projectId, userId sharedTypes.UUID, f *Folder) (sharedTypes.Version, []Doc, error)
	GetAuthorizationDetails(ctx context.Context, projectId, userId sharedTypes.UUID, accessToken AccessToken) (*AuthorizationDetails, error)
	GetForProjectJWT(ctx context.Context, projectId, userId sharedTypes.UUID, accessToken AccessToken) (*ForProjectJWT, int64, error)
	GetForZip(ctx context.Context, projectId sharedTypes.UUID, epoch int64) (*ForZip, error)
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
	GetProjectAccessForReadAndWriteToken(ctx context.Context, userId sharedTypes.UUID, accessToken AccessToken) (*TokenAccessResult, error)
	GetProjectAccessForReadOnlyToken(ctx context.Context, userId sharedTypes.UUID, accessToken AccessToken) (*TokenAccessResult, error)
	GetEntries(ctx context.Context, projectId, userId sharedTypes.UUID) (*ForProjectEntries, error)
	GetProjectMembers(ctx context.Context, projectId sharedTypes.UUID) ([]user.AsProjectMember, error)
	GrantMemberAccess(ctx context.Context, projectId sharedTypes.UUID, epoch int64, userId sharedTypes.UUID, level sharedTypes.PrivilegeLevel) error
	GrantReadAndWriteTokenAccess(ctx context.Context, projectId sharedTypes.UUID, epoch int64, userId sharedTypes.UUID) error
	GrantReadOnlyTokenAccess(ctx context.Context, projectId sharedTypes.UUID, epoch int64, userId sharedTypes.UUID) error
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
	RemoveMember(ctx context.Context, projectId sharedTypes.UUID, epoch int64, userId sharedTypes.UUID) error
	TransferOwnership(ctx context.Context, projectId, previousOwnerId, newOwnerId sharedTypes.UUID) (*user.WithPublicInfo, *user.WithPublicInfo, Name, error)
	CreateDoc(ctx context.Context, projectId, userId, folderId sharedTypes.UUID, d *Doc) (sharedTypes.Version, error)
	CreateFile(ctx context.Context, projectId, userId, folderId sharedTypes.UUID, f *FileRef) (sharedTypes.Version, error)
	ListProjects(ctx context.Context, userId sharedTypes.UUID, r *ForProjectList) error
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
WITH lng AS (SELECT CASE
                        WHEN $2 = 'inherit' THEN
                            (editor_config->'spellCheckLanguage')::text
                        ELSE
                            $2::text
                        END AS spell_check_language
             FROM users
             WHERE id = $1),
     p AS (INSERT
         INTO projects
             (compiler, deleted_at, epoch, id, image_name, last_opened_at,
              last_updated_at, last_updated_by, name, owner_id,
              public_access_level, spell_check_language, tree_version)
             SELECT $3,
                    $4,
                    1,
                    $5,
                    $6,
                    transaction_timestamp(),
                    transaction_timestamp(),
                    $1,
                    $7,
                    $1,
                    '',
                    lng.spell_check_language,
                    1
             FROM lng)
INSERT
INTO project_members
(project_id, user_id, can_write, is_token_member, archived, trashed)
VALUES ($5, $1, TRUE, FALSE, FALSE, FALSE)
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
	_, err := m.db.ExecContext(ctx, `
UPDATE projects
SET deleted_at     = NULL,
    name           = $2,
    root_doc_id    = $3,
    root_folder_id = $4
WHERE id = $1
`, p.Id.String(), p.Name, rootDocId, p.RootFolder.Id.String())
	return err
}

type genericExistsAndAuthResult struct {
	ProjectExists bool `edgedb:"project_exists"`
	AuthCheck     bool `edgedb:"auth_check"`
	OK            bool `edgedb:"ok"`
}

func (r genericExistsAndAuthResult) toError() error {
	switch {
	case !r.ProjectExists:
		return &errors.NotFoundError{}
	case !r.AuthCheck:
		return &errors.NotAuthorizedError{}
	}
	return nil
}

func (m *manager) PopulateTokens(ctx context.Context, projectId, userId sharedTypes.UUID) (*Tokens, error) {
	allErrors := &errors.MergedError{}
	for i := 0; i < 10; i++ {
		tokens, err := generateTokens()
		if err != nil {
			allErrors.Add(err)
			continue
		}
		r := genericExistsAndAuthResult{}
		err = m.c.QuerySingle(ctx, `
with
	u := (select User filter .id = <uuid>$1 and not exists .deleted_at),
	p := (select Project filter .id = <uuid>$0 and not exists .deleted_at),
	pWithAuth := (select p filter .owner = u),
	created := (
		update pWithAuth
		filter (.tokens.token_ro ?? '') = ''
		set {
			tokens := (insert Tokens {
				token_ro := <str>$2,
				token_rw := <str>$3,
				token_prefix_rw := <str>$4,
			}),
		}
	)
select {
	project_exists := exists p,
	auth_check := exists pWithAuth,
	ok := exists created,
}
`,
			&r,
			projectId, userId,
			tokens.ReadOnly, tokens.ReadAndWrite, tokens.ReadAndWritePrefix,
		)
		if err != nil {
			if e, ok := err.(edgedb.Error); ok && e.Category(edgedb.ConstraintViolationError) {
				allErrors.Add(err)
				continue
			}
			return nil, rewriteEdgedbError(err)
		}
		if err = r.toError(); err != nil {
			return nil, err
		}
		if r.OK {
			return tokens, nil
		}
		// tokens are already populated
		return nil, nil
	}
	return nil, errors.Tag(allErrors, "bad random source")
}

func (m *manager) SetCompiler(ctx context.Context, projectId, userId sharedTypes.UUID, compiler sharedTypes.Compiler) error {
	r := genericExistsAndAuthResult{}
	err := m.c.QuerySingle(ctx, `
with
	u := (select User filter .id = <uuid>$1 and not exists .deleted_at),
	p := (select Project filter .id = <uuid>$0 and not exists .deleted_at),
	pWithAuth := (select p filter u in .min_access_rw),
	updated := (update pWithAuth set { compiler := <str>$2 })
select {
	project_exists := exists p,
	auth_check := exists pWithAuth,
	ok := exists updated,
}
`, &r, projectId, userId, string(compiler))
	if err != nil {
		return rewriteEdgedbError(err)
	}
	return r.toError()
}

func (m *manager) SetImageName(ctx context.Context, projectId, userId sharedTypes.UUID, imageName sharedTypes.ImageName) error {
	r := genericExistsAndAuthResult{}
	err := m.c.QuerySingle(ctx, `
with
	u := (select User filter .id = <uuid>$1 and not exists .deleted_at),
	p := (select Project filter .id = <uuid>$0 and not exists .deleted_at),
	pWithAuth := (select p filter u in .min_access_rw),
	updated := (update pWithAuth set { image_name := <str>$2 })
select {
	project_exists := exists p,
	auth_check := exists pWithAuth,
	ok := exists updated,
}
`, &r, projectId, userId, string(imageName))
	if err != nil {
		return rewriteEdgedbError(err)
	}
	return r.toError()
}

func (m *manager) SetSpellCheckLanguage(ctx context.Context, projectId, userId sharedTypes.UUID, spellCheckLanguage spellingTypes.SpellCheckLanguage) error {
	r := genericExistsAndAuthResult{}
	err := m.c.QuerySingle(ctx, `
with
	u := (select User filter .id = <uuid>$1 and not exists .deleted_at),
	p := (select Project filter .id = <uuid>$0 and not exists .deleted_at),
	pWithAuth := (select p filter u in .min_access_rw),
	updated := (update pWithAuth set { spell_check_language := <str>$2 })
select {
	project_exists := exists p,
	auth_check := exists pWithAuth,
	ok := exists updated,
}
`, &r, projectId, userId, string(spellCheckLanguage))
	if err != nil {
		return rewriteEdgedbError(err)
	}
	return r.toError()
}

type setRootDocResult struct {
	ProjectExists   bool `edgedb:"project_exists"`
	AuthCheck       bool `edgedb:"auth_check"`
	DocExists       bool `edgedb:"doc_exists"`
	RootDocEligible bool `edgedb:"root_doc_eligible"`
	ProjectUpdated  bool `edgedb:"project_updated"`
}

func (m *manager) SetRootDoc(ctx context.Context, projectId, userId, rootDocId sharedTypes.UUID) error {
	r := setRootDocResult{}
	err := m.c.QuerySingle(ctx, `
with
	p := (select Project filter .id = <uuid>$0 and not exists .deleted_at),
	u := (select User filter .id = <uuid>$2 and not exists .deleted_at),
	pWithAuth := (select p filter u in .min_access_rw),
	d := (
		select Doc
		filter .id = <uuid>$1 and not .deleted and .project = pWithAuth
	),
	newRootDoc := (
		select d
		filter (
		   .name LIKE '%.tex' or .name LIKE '%.rtex' or .name LIKE '%.ltex'
		)
	),
	pUpdated := (update newRootDoc.project set { root_doc := d })
select {
	project_exists := exists p,
	auth_check := exists pWithAuth,
	doc_exists := exists d,
	root_doc_eligible := exists newRootDoc,
	project_updated := exists pUpdated,
}
`, &r, projectId, rootDocId, userId)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	switch {
	case !r.ProjectExists:
		return &errors.NotFoundError{}
	case !r.AuthCheck:
		return &errors.NotAuthorizedError{}
	case !r.DocExists:
		return &errors.UnprocessableEntityError{
			Msg: "doc does not exist",
		}
	case !r.RootDocEligible:
		return &errors.UnprocessableEntityError{
			Msg: "doc does not have root doc extension (.tex, .rtex, .ltex)",
		}
	default:
		// project updated
		return nil
	}
}

func (m *manager) SetPublicAccessLevel(ctx context.Context, projectId, userId sharedTypes.UUID, publicAccessLevel PublicAccessLevel) error {
	r := genericExistsAndAuthResult{}
	err := m.c.QuerySingle(ctx, `
with
	u := (select User filter .id = <uuid>$1 and not exists .deleted_at),
	p := (select Project filter .id = <uuid>$0 and not exists .deleted_at),
	pWithAuth := (select p filter .owner = u),
	updated := (
		update pWithAuth
		filter .tokens.token_ro != ''
		set { public_access_level := <str>$2 }
	)
select {
	project_exists := exists p,
	auth_check := exists pWithAuth,
	ok := exists updated,
}
`, &r, projectId, userId, string(publicAccessLevel))
	if err != nil {
		return rewriteEdgedbError(err)
	}
	if err = r.toError(); err != nil {
		return err
	}
	if !r.OK {
		return &errors.UnprocessableEntityError{Msg: "missing tokens"}
	}
	return nil
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
	r := genericExistsAndAuthResult{}
	err := m.c.QuerySingle(ctx, `
with
	u := (select User filter .id = <uuid>$1 and not exists .deleted_at),
	p := (select Project filter .id = <uuid>$0 and not exists .deleted_at),
	pWithAuth := (select p filter .owner = u),
	updated := (update pWithAuth set { name := <str>$2 })
select {
	project_exists := exists p,
	auth_check := exists pWithAuth,
	ok := exists updated,
}
`,
		&r,
		projectId, userId, string(name),
	)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	return r.toError()
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
          AND pm.can_write = TRUE
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

type genericTreeElementResult struct {
	genericExistsAndAuthResult `edgedb:"$inline"`
	ElementExists              bool                `edgedb:"element_exists"`
	ProjectVersion             sharedTypes.Version `edgedb:"project_version"`
}

func (r genericTreeElementResult) toError() error {
	if err := r.genericExistsAndAuthResult.toError(); err != nil {
		return err
	}
	if !r.ElementExists {
		return &errors.UnprocessableEntityError{
			Msg: "element does not exist",
		}
	}
	return nil
}

func (m *manager) DeleteDoc(ctx context.Context, projectId, userId, docId sharedTypes.UUID) (sharedTypes.Version, error) {
	r := genericTreeElementResult{}
	err := m.c.QuerySingle(ctx, `
with
	p := (select Project filter .id = <uuid>$0),
	u := (select User filter .id = <uuid>$1),
	pWithAuth := (select p filter u in .min_access_rw),
	d := (
		select Doc
		filter
			.id = <uuid>$2
		and .project = pWithAuth
		and not .deleted
	),
	deletedDoc := (update d set {
		deleted_at := datetime_of_transaction(),
	}),
	pBumpedVersion := (update deletedDoc.project set {
		version := deletedDoc.project.version + 1,
		root_doc := (
			<Doc>{}
			if (deletedDoc.project.root_doc = d) else
			deletedDoc.project.root_doc
		),
		last_updated_at := datetime_of_transaction(),
		last_updated_by := u,
	})
select {
	project_exists := exists p,
	auth_check := exists pWithAuth,
	element_exists := exists d,
	project_version := pBumpedVersion.version ?? 0,
}
`, &r, projectId, userId, docId)
	if err != nil {
		return 0, rewriteEdgedbError(err)
	}
	return r.ProjectVersion, r.toError()
}

func (m *manager) DeleteFile(ctx context.Context, projectId, userId, fileId sharedTypes.UUID) (sharedTypes.Version, error) {
	r := genericTreeElementResult{}
	err := m.c.QuerySingle(ctx, `
with
	p := (select Project filter .id = <uuid>$0),
	u := (select User filter .id = <uuid>$1),
	pWithAuth := (select p filter u in .min_access_rw),
	f := (
		select File
		filter
			.id = <uuid>$2
		and .project = pWithAuth
		and not .deleted
	),
	deletedFile := (update f set {
		deleted_at := datetime_of_transaction(),
	}),
	pBumpedVersion := (update deletedFile.project set {
		version := deletedFile.project.version + 1,
		last_updated_at := datetime_of_transaction(),
		last_updated_by := u,
	})
select {
	project_exists := exists p,
	auth_check := exists pWithAuth,
	element_exists := exists f,
	project_version := pBumpedVersion.version ?? 0,
}
`, &r, projectId, userId, fileId)
	if err != nil {
		return 0, rewriteEdgedbError(err)
	}
	return r.ProjectVersion, r.toError()
}

type deletedFolderResult struct {
	genericTreeElementResult `edgedb:"$inline"`
	DeletedChildren          bool `edgedb:"deleted_children"`
}

func (m *manager) DeleteFolder(ctx context.Context, projectId, userId, folderId sharedTypes.UUID) (sharedTypes.Version, error) {
	r := deletedFolderResult{}
	err := m.c.QuerySingle(ctx, `
with
	p := (select Project filter .id = <uuid>$0),
	u := (select User filter .id = <uuid>$1),
	pWithAuth := (select p filter u in .min_access_rw),
	f := (
		select Folder
		filter
			.id = <uuid>$2
		and .project = pWithAuth
		and not .deleted
	),
	deletedFolder := (update f set {
		deleted_at := datetime_of_transaction(),
	}),
	deletionPrefix := deletedFolder.path ++ '/%',
	deletedItems := (
		update VisibleTreeElement
		filter
			.project = pWithAuth
		and not .deleted
		and .parent.path_for_join like deletionPrefix
		set {
			deleted_at := datetime_of_transaction(),
		}
	),
	pBumpedVersion := (update deletedFolder.project set {
		version := deletedFolder.project.version + 1,
		root_doc := (
			<Doc>{} if (
				deletedFolder.project.root_doc.resolved_path
				like deletionPrefix
			) else deletedFolder.project.root_doc
		),
		last_updated_at := datetime_of_transaction(),
		last_updated_by := u,
	})
select {
	project_exists := exists p,
	auth_check := exists pWithAuth,
	element_exists := exists f,
	deleted_children := exists deletedItems,
	project_version := pBumpedVersion.version ?? 0,
}
`, &r, projectId, userId, folderId)
	if err != nil {
		return 0, rewriteEdgedbError(err)
	}
	return r.ProjectVersion, r.toError()
}

type moveTreeElementResult struct {
	genericTreeElementResult `edgedb:"$inline"`
	NewPath                  sharedTypes.PathName `edgedb:"new_path"`
}

func (m *manager) MoveDoc(ctx context.Context, projectId, userId, folderId, docId sharedTypes.UUID) (sharedTypes.Version, sharedTypes.PathName, error) {
	r := moveTreeElementResult{}
	err := m.c.QuerySingle(ctx, `
with
	p := (select Project filter .id = <uuid>$0),
	u := (select User filter .id = <uuid>$1),
	pWithAuth := (select p filter u in .min_access_rw),
	d := (select Doc filter .id = <uuid>$3 and .project = pWithAuth),
	updatedDoc := (update d set {
		parent := (
			select FolderLike
			filter .id = <uuid>$2 and .project = pWithAuth
		),
	}),
	pBumpedVersion := (update updatedDoc.project set {
		version := updatedDoc.project.version + 1,
		last_updated_at := datetime_of_transaction(),
		last_updated_by := u,
	})
select {
	project_exists := exists p,
	auth_check := exists pWithAuth,
	element_exists := exists d,
	new_path := updatedDoc.resolved_path ?? '',
	project_version := pBumpedVersion.version ?? 0,
}
`, &r, projectId, userId, folderId, docId)
	if err != nil {
		return 0, "", rewriteEdgedbError(err)
	}
	return r.ProjectVersion, r.NewPath, r.toError()
}

func (m *manager) MoveFile(ctx context.Context, projectId, userId, folderId, fileId sharedTypes.UUID) (sharedTypes.Version, error) {
	r := moveTreeElementResult{}
	err := m.c.QuerySingle(ctx, `
with
	p := (select Project filter .id = <uuid>$0),
	u := (select User filter .id = <uuid>$1),
	pWithAuth := (select p filter u in .min_access_rw),
	f := (select File filter .id = <uuid>$3 and .project = pWithAuth),
	updatedFile := (update f set {
		parent := (
			select FolderLike
			filter .id = <uuid>$2 and .project = pWithAuth
		),
	}),
	pBumpedVersion := (update updatedFile.project set {
		version := updatedFile.project.version + 1,
		last_updated_at := datetime_of_transaction(),
		last_updated_by := u,
	})
select {
	project_exists := exists p,
	auth_check := exists pWithAuth,
	element_exists := exists f,
	project_version := pBumpedVersion.version ?? 0,
}
`, &r, projectId, userId, folderId, fileId)
	if err != nil {
		return 0, rewriteEdgedbError(err)
	}
	return r.ProjectVersion, r.toError()
}

type moveFolderResult struct {
	moveTreeElementResult `edgedb:"$inline"`
	DocsField             `edgedb:"$inline"`
	TargetExists          bool `edgedb:"target_exists"`
	TargetLoopCheck       bool `edgedb:"target_loop_check"`
}

func (m *manager) MoveFolder(ctx context.Context, projectId, userId, targetFolderId, folderId sharedTypes.UUID) (sharedTypes.Version, []Doc, error) {
	r := moveFolderResult{}
	err := m.c.QuerySingle(ctx, `
with
	p := (select Project filter .id = <uuid>$0),
	u := (select User filter .id = <uuid>$1),
	pWithAuth := (select p filter u in .min_access_rw),
	f := (select Folder filter .id = <uuid>$3 and .project = pWithAuth),
	oldPath := f.path,
	target := (
		select FolderLike filter .id = <uuid>$2 and .project = pWithAuth
	),
	targetWithLoopCheck := (
		select FolderLike
		filter
			FolderLike = target
		and .path_for_join not like (oldPath ++ '/%')
	),
	newPath := targetWithLoopCheck.path_for_join ++ f.name,
	updatedFolders := (update Folder
		filter
			.project = targetWithLoopCheck.project
		and .path_for_join like (oldPath ++ '/%')
		set {
			parent := (targetWithLoopCheck if (Folder = f) else Folder.parent),
			path := newPath ++ Folder.path[len(oldPath):],
		}
	),
	pBumpedVersion := (
		update targetWithLoopCheck.project
		set {
			version := targetWithLoopCheck.project.version + 1,
			last_updated_at := datetime_of_transaction(),
			last_updated_by := u,
		}
	)
select {
	project_exists := exists p,
	auth_check := exists pWithAuth,
	element_exists := exists f,
	target_exists := exists target,
	target_loop_check := exists targetWithLoopCheck,
	project_version := pBumpedVersion.version ?? 0,
	docs := (select updatedFolders.docs { id, resolved_path })
}
`, &r, projectId, userId, targetFolderId, folderId)
	if err != nil {
		return 0, nil, rewriteEdgedbError(err)
	}
	if err = r.toError(); err != nil {
		return 0, nil, err
	}
	switch {
	case !r.TargetExists:
		return 0, nil, &errors.UnprocessableEntityError{
			Msg: "target does not exist",
		}
	case !r.TargetLoopCheck:
		return 0, nil, &errors.UnprocessableEntityError{
			Msg: "target is inside folder",
		}
	}
	return r.ProjectVersion, r.Docs, nil
}

type renameTreeElementResult struct {
	genericTreeElementResult `edgedb:"$inline"`
	ParentPath               sharedTypes.DirName `edgedb:"parent_path"`
}

func (m *manager) RenameDoc(ctx context.Context, projectId, userId sharedTypes.UUID, d *Doc) (sharedTypes.Version, sharedTypes.DirName, error) {
	r := renameTreeElementResult{}
	err := m.c.QuerySingle(ctx, `
with
	p := (select Project filter .id = <uuid>$0),
	u := (select User filter .id = <uuid>$1),
	pWithAuth := (select p filter u in .min_access_rw),
	d := (select Doc filter .id = <uuid>$2 and .project = pWithAuth),
	updatedDoc := (update d set { name := <str>$3 }),
	pBumpedVersion := (
		update updatedDoc.project
		set {
			version := pWithAuth.version + 1,
			last_updated_at := datetime_of_transaction(),
			last_updated_by := u,
		}
	)
select {
	project_exists := exists p,
	auth_check := exists pWithAuth,
	element_exists := exists d,
	parent_path := updatedDoc.parent.path ?? '',
	project_version := pBumpedVersion.version ?? 0,
}
`, &r, projectId, userId, d.Id, d.Name)
	if err != nil {
		return 0, "", rewriteEdgedbError(err)
	}
	return r.ProjectVersion, r.ParentPath, r.toError()
}

func (m *manager) RenameFile(ctx context.Context, projectId, userId sharedTypes.UUID, f *FileRef) (sharedTypes.Version, error) {
	r := renameTreeElementResult{}
	err := m.c.QuerySingle(ctx, `
with
	p := (select Project filter .id = <uuid>$0),
	u := (select User filter .id = <uuid>$1),
	pWithAuth := (select p filter u in .min_access_rw),
	f := (select File filter .id = <uuid>$2 and .project = pWithAuth),
	updatedFile := (update f set { name := <str>$3 }),
	pBumpedVersion := (
		update updatedFile.project
		set {
			version := pWithAuth.version + 1,
			last_updated_at := datetime_of_transaction(),
			last_updated_by := u,
		}
	)
select {
	project_exists := exists p,
	auth_check := exists pWithAuth,
	element_exists := exists f,
	project_version := pBumpedVersion.version ?? 0,
}
`, &r, projectId, userId, f.Id, f.Name)
	if err != nil {
		return 0, rewriteEdgedbError(err)
	}
	return r.ProjectVersion, r.toError()
}

type renameFolderResult struct {
	renameTreeElementResult `edgedb:"$inline"`
	DocsField               `edgedb:"$inline"`
}

func (m *manager) RenameFolder(ctx context.Context, projectId, userId sharedTypes.UUID, f *Folder) (sharedTypes.Version, []Doc, error) {
	r := renameFolderResult{}
	err := m.c.QuerySingle(ctx, `
with
	p := (select Project filter .id = <uuid>$0),
	u := (select User filter .id = <uuid>$1),
	pWithAuth := (select p filter u in .min_access_rw),
	f := (select Folder filter .id = <uuid>$2 and .project = pWithAuth),
	updatedFolders := (update Folder
		filter
			.project = pWithAuth
		and .path_for_join like (f.path ++ '/%')
		set {
			name := (<str>$3 if (Folder = f) else Folder.name),
			path := (
				f.path[:-len(f.name)]
				++ <str>$3
				++ Folder.path[len(f.path):]
			)
		}
	),
	pBumpedVersion := (
		update f.project
		set {
			version := f.project.version + 1,
			last_updated_at := datetime_of_transaction(),
			last_updated_by := u,
		}
	)
select {
	project_exists := exists p,
	auth_check := exists pWithAuth,
	element_exists := exists f,
	parent_path := f.parent.path ?? '',
	project_version := pBumpedVersion.version ?? 0,
	docs := (select updatedFolders.docs { id, resolved_path }),
}
`, &r, projectId, userId, f.Id, f.Name)
	if err != nil {
		return 0, nil, rewriteEdgedbError(err)
	}
	return r.ProjectVersion, r.Docs, r.toError()
}

func (m *manager) ArchiveForUser(ctx context.Context, projectId, userId sharedTypes.UUID) error {
	r := genericExistsAndAuthResult{}
	err := m.c.QuerySingle(ctx, `
with
	u := (select User filter .id = <uuid>$1 and not exists .deleted_at),
	p := (select Project filter .id = <uuid>$0 and not exists .deleted_at),
	pWithAuth := (select p filter u in .min_access_ro),
	updated := (
		update pWithAuth
		set {
			archived_by := distinct (pWithAuth.archived_by union {u}),
			trashed_by -= u
		}
	)
select {
	project_exists := exists p,
	auth_check := exists pWithAuth,
	ok := exists updated,
}
`, &r, projectId, userId)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	return r.toError()
}

func (m *manager) UnArchiveForUser(ctx context.Context, projectId, userId sharedTypes.UUID) error {
	r := genericExistsAndAuthResult{}
	err := m.c.QuerySingle(ctx, `
with
	u := (select User filter .id = <uuid>$1 and not exists .deleted_at),
	p := (select Project filter .id = <uuid>$0 and not exists .deleted_at),
	pWithAuth := (select p filter u in .min_access_ro),
	updated := (update pWithAuth set { archived_by -= u })
select {
	project_exists := exists p,
	auth_check := exists pWithAuth,
	ok := exists updated,
}
`, &r, projectId, userId)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	return r.toError()
}

func (m *manager) TrashForUser(ctx context.Context, projectId, userId sharedTypes.UUID) error {
	r := genericExistsAndAuthResult{}
	err := m.c.QuerySingle(ctx, `
with
	u := (select User filter .id = <uuid>$1 and not exists .deleted_at),
	p := (select Project filter .id = <uuid>$0 and not exists .deleted_at),
	pWithAuth := (select p filter u in .min_access_ro),
	updated := (
		update pWithAuth
		set {
			trashed_by := distinct (pWithAuth.trashed_by union {u}),
			archived_by -= u
		}
	)
select {
	project_exists := exists p,
	auth_check := exists pWithAuth,
	ok := exists updated,
}
`, &r, projectId, userId)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	return r.toError()
}

func (m *manager) UnTrashForUser(ctx context.Context, projectId, userId sharedTypes.UUID) error {
	r := genericExistsAndAuthResult{}
	err := m.c.QuerySingle(ctx, `
with
	u := (select User filter .id = <uuid>$1 and not exists .deleted_at),
	p := (select Project filter .id = <uuid>$0 and not exists .deleted_at),
	pWithAuth := (select p filter u in .min_access_ro),
	updated := (update pWithAuth set { trashed_by -= u })
select {
	project_exists := exists p,
	auth_check := exists pWithAuth,
	ok := exists updated,
}
`, &r, projectId, userId)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	return r.toError()
}

var ErrEpochIsNotStable = errors.New("epoch is not stable")

func (m *manager) GetProjectNames(ctx context.Context, userId sharedTypes.UUID) (Names, error) {
	var raw []string
	err := m.db.QueryRowContext(ctx, `
SELECT array_agg(name)
FROM projects p
	INNER JOIN project_members pm on p.id = pm.project_id
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
SELECT COALESCE(can_write, FALSE),
       epoch,
       COALESCE(is_token_member, TRUE),
       owner_id,
       public_access_level,
       COALESCE(p.token_ro, ''),
       COALESCE(p.token_rw, '')
FROM projects p
         LEFT JOIN project_members pm ON (p.id = pm.project_id AND
                                          pm.user_id = $2)
WHERE p.id = $1
  AND (
        (pm.is_token_member = FALSE)
        OR (p.public_access_level = 'tokenBased' AND pm.is_token_member = TRUE)
        OR (p.public_access_level = 'tokenBased' AND p.token_ro = $3)
    )
`, projectId, userId, accessToken).Scan(
		&p.CanWrite, &p.Epoch, &p.IsTokenMember, &p.OwnerId,
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
SELECT COALESCE(pm.can_write, FALSE),
       p.epoch,
       COALESCE(pm.is_token_member, TRUE),
       p.owner_id,
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
        (pm.is_token_member = FALSE)
        OR (p.public_access_level = 'tokenBased' AND pm.is_token_member = TRUE)
        OR (p.public_access_level = 'tokenBased' AND p.token_ro = $3)
    )
`, projectId, userId, accessToken).Scan(
		&p.CanWrite,
		&p.Epoch,
		&p.IsTokenMember,
		&p.OwnerId,
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
         INNER JOIN projects p on t.project_id = p.id
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

type restoreDocResult struct {
	genericTreeElementResult `edgedb:"$inline"`
	DocDeleted               bool `edgedb:"doc_deleted"`
	Doc                      struct {
		DocWithParent `edgedb:"$inline"`
	} `edgedb:"doc"`
}

func (m *manager) RestoreDoc(ctx context.Context, projectId, userId, docId sharedTypes.UUID, name sharedTypes.Filename) (*DocWithParent, sharedTypes.Version, error) {
	r := restoreDocResult{}
	err := m.c.QuerySingle(ctx, `
with
	u := (select User filter .id = <uuid>$1 and not exists .deleted_at),
	p := (select Project filter .id = <uuid>$0 and not exists .deleted_at),
	pWithAuth := (select p filter u in .min_access_rw),
	d := (select Doc filter .id = <uuid>$2 and .project = pWithAuth),
	dWithDeletedCheck := (select d filter .deleted),
	dRestored := (
		update dWithDeletedCheck
		set {
			name := <str>$3,
			deleted_at := <datetime>'1970-01-01T00:00:00.000000Z',
		}
	),
	pBumped := (
		update dRestored.project
		set {
			version := dRestored.project.version + 1,
			last_updated_at := datetime_of_transaction(),
			last_updated_by := u,
		}
	)
select {
	project_exists := exists p,
	auth_check := exists pWithAuth,
	element_exists := exists d,
	doc_deleted := exists dWithDeletedCheck,
	doc := dRestored {
		id,
		name,
		parent,
	},
	project_version := pBumped.version,
`, &r, projectId, userId, docId, name)
	if err != nil {
		if e, ok := err.(edgedb.Error); ok && e.Category(edgedb.ConstraintViolationError) {
			return nil, 0, ErrDuplicateNameInFolder
		}
		return nil, 0, rewriteEdgedbError(err)
	}
	if err = r.toError(); err != nil {
		return nil, 0, err
	}
	if !r.DocDeleted {
		return nil, 0, &errors.UnprocessableEntityError{Msg: "doc not deleted"}
	}
	return &r.Doc.DocWithParent, r.ProjectVersion, nil
}

func (m *manager) GetFile(ctx context.Context, projectId, userId sharedTypes.UUID, accessToken AccessToken, fileId sharedTypes.UUID) (*FileWithParent, error) {
	f := FileWithParent{}
	d := LinkedFileData{}
	err := m.db.QueryRowContext(ctx, `
SELECT t.path, t.parent_id, f.linked_file_data, f.size
FROM files f
         INNER JOIN tree_nodes t ON f.id = t.id
         INNER JOIN projects p on t.project_id = p.id
         LEFT JOIN project_members pm ON (p.id = pm.project_id AND
                                          pm.user_id = $2)
WHERE f.id = $4
  AND t.project_id = $1
  AND t.deleted_at = '1970-01-01'
  AND p.deleted_at IS NULL
  AND (
        (pm.is_token_member = FALSE)
        OR (p.public_access_level = 'tokenBased' AND pm.is_token_member = TRUE)
        OR (p.public_access_level = 'tokenBased' AND p.token_ro = $3)
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

type getDocOrFileResult struct {
	genericExistsAndAuthResult `edgedb:"$inline"`
	ParentExists               bool                `edgedb:"parent_exists"`
	IsDoc                      bool                `edgedb:"is_doc"`
	IsFolder                   bool                `edgedb:"is_folder"`
	ElementId                  edgedb.OptionalUUID `edgedb:"element_id"`
}

func (m *manager) GetElementHintForOverwrite(ctx context.Context, projectId, userId, folderId sharedTypes.UUID, name sharedTypes.Filename) (sharedTypes.UUID, bool, error) {
	r := getDocOrFileResult{}
	err := m.c.QuerySingle(ctx, `
with
	p := (select Project filter .id = <uuid>$0),
	u := (select User filter .id = <uuid>$1),
	pWithAuth := (select p filter u in .min_access_rw),
	f := (select FolderLike filter .id = <uuid>$2 and .project = pWithAuth),
	e := (
		select VisibleTreeElement
		filter .parent = f and .name = <str>$3 and not .deleted
		limit 1
	),
select {
	project_exists := exists p,
	auth_check := exists pWithAuth,
	parent_exists := exists f,
	is_doc := e is Doc ?? false,
	is_folder := e is Folder ?? false,
	element_id := e.id,
}
`, &r, projectId, userId, folderId, name)
	if err != nil {
		return sharedTypes.UUID{}, false, rewriteEdgedbError(err)
	}
	if err = r.toError(); err != nil {
		return sharedTypes.UUID{}, false, err
	}
	switch {
	case !r.ParentExists:
		return sharedTypes.UUID{}, false, &errors.UnprocessableEntityError{
			Msg: "parent folder does not exist",
		}
	case r.IsFolder:
		return sharedTypes.UUID{}, false, &errors.UnprocessableEntityError{
			Msg: "element is a folder",
		}
	}
	id, _ := r.ElementId.Get()
	return sharedTypes.UUID(id), r.IsDoc, nil
}

func (m *manager) GetElementByPath(ctx context.Context, projectId, userId sharedTypes.UUID, path sharedTypes.PathName) (sharedTypes.UUID, bool, error) {
	r := getDocOrFileResult{}
	err := m.c.QuerySingle(ctx, `
with
	p := (select Project filter .id = <uuid>$0 and not exists .deleted_at),
	u := (select User filter .id = <uuid>$1 and not exists .deleted_at),
	pWithAuth := (select p filter u in .min_access_ro),
	e := (
		select ContentElement
		filter
			.project = pWithAuth
 		and not .deleted
		and .resolved_path = <str>$2
		limit 1
	),
select {
	project_exists := exists p,
	auth_check := exists pWithAuth,
	is_doc := e is Doc ?? false,
	element_id := e.id,
}
`, &r, projectId, userId, path)
	if err != nil {
		return sharedTypes.UUID{}, false, rewriteEdgedbError(err)
	}
	if err = r.toError(); err != nil {
		return sharedTypes.UUID{}, false, err
	}
	id, exists := r.ElementId.Get()
	if !exists {
		return sharedTypes.UUID{}, false, &errors.NotFoundError{}
	}
	return sharedTypes.UUID(id), r.IsDoc, nil
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

func (m *manager) GetForZip(ctx context.Context, projectId sharedTypes.UUID, epoch int64) (*ForZip, error) {
	p := &ForZip{}
	err := m.c.QuerySingle(
		ctx,
		`
select Project {
	folders: {
		id,
		name,
		folders,
		docs: { id, name, snapshot },
		files: { id, name },
	},
	name,
	root_folder: {
		id,
		folders,
		docs: { id, name, snapshot },
		files: { id, name },
	},
}
filter .id = <uuid>$0 and .epoch = <int64>$1 and not exists .deleted_at
`,
		p,
		projectId, epoch,
	)
	if err != nil {
		err = rewriteEdgedbError(err)
		if errors.IsNotFoundError(err) {
			return nil, ErrEpochIsNotStable
		}
		return nil, err
	}
	return p, nil
}

func (m *manager) GetJoinProjectDetails(ctx context.Context, projectId, userId sharedTypes.UUID, accessToken AccessToken) (*JoinProjectDetails, error) {
	d := &JoinProjectDetails{}
	d.Project.Id = projectId
	d.Project.RootFolder.Folder = NewFolder("")
	d.Project.DeletedDocs = make([]CommonTreeFields, 0)

	var treeIds []sharedTypes.UUID
	var treeKinds []string
	var treePaths []string
	var deletedDocIds []sharedTypes.UUID
	var deletedDocNames []string

	// TODO: fetch file details `created_at` and `linked_file_data`
	// TODO: let frontend query members/invites on modal open (again)
	err := m.db.QueryRowContext(ctx, `
WITH tree AS
         (SELECT t.project_id,
                 array_remove(array_agg(t.id), NULL)        as ids,
                 array_remove(array_agg(t.kind), NULL)      as kinds,
                 array_remove(array_agg(t.path), NULL)      as paths
          FROM tree_nodes t
          WHERE t.project_id = $1
            AND t.deleted_at = '1970-01-01'
			AND t.parent_id IS NOT NULL
          GROUP BY t.project_id),
     deleted_docs AS (SELECT t.project_id,
                             array_remove(array_agg(t.id), NULL)   as ids,
                             array_remove(array_agg(t.name), NULL) as names
                      FROM tree_nodes t
                      WHERE t.project_id = $1
                        AND t.deleted_at != '1970-01-01'
                      GROUP BY t.project_id)

SELECT p.compiler,
       p.epoch,
       p.image_name,
       p.name,
       p.owner_id,
       p.public_access_level,
       p.root_doc_id,
       p.root_folder_id,
       p.spell_check_language,
       COALESCE(p.token_ro, ''),
       COALESCE(p.token_rw, ''),
       p.tree_version,
       o.features,
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
        (pm.is_token_member = FALSE)
        OR (p.public_access_level = 'tokenBased' AND pm.is_token_member = TRUE)
        OR (p.public_access_level = 'tokenBased' AND p.token_ro = $3)
    )
`, projectId, userId, accessToken).Scan(
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
		pq.Array(&treeIds),
		pq.Array(&treeKinds),
		pq.Array(&treePaths),
		pq.Array(&deletedDocIds),
		pq.Array(&deletedDocNames),
	)
	if err != nil {
		return nil, err
	}

	t := &d.Project.RootFolder
	for i, kind := range treeKinds {
		p := sharedTypes.PathName(treePaths[i])
		f, err2 := t.CreateParents(p.Dir())
		if err2 != nil {
			return nil, errors.Tag(err2, strconv.Itoa(i))
		}
		switch kind {
		case "doc":
			e := NewDoc(p.Filename())
			e.Id = treeIds[i]
			f.Docs = append(f.Docs, e)
		case "file":
			e := NewFileRef(p.Filename(), "", 0)
			e.Id = treeIds[i]
			f.FileRefs = append(f.FileRefs, e)
		case "folder":
			// NOTE: The paths of folders have a trailing slash in the DB.
			//       When getting f, that slash is removed by the p.Dir() call
			//        and f will have the correct path/name. :)
			f.Id = treeIds[i]
		}
	}
	for i, id := range deletedDocIds {
		d.Project.DeletedDocs = append(d.Project.DeletedDocs, CommonTreeFields{
			Id:   id,
			Name: sharedTypes.Filename(deletedDocNames[i]),
		})
	}
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
SELECT p.compiler,
       p.epoch,
       p.image_name,
       p.name,
       p.owner_id,
       p.public_access_level,
       COALESCE(p.token_ro, ''),
       COALESCE(p.token_rw, ''),
       p.tree_version,
       d.id,
       d.path,
       o.features,
       u.editor_config,
       u.email,
       u.epoch,
       u.first_name,
       u.last_name
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
        (pm.is_token_member = FALSE)
        OR (p.public_access_level = 'tokenBased' AND pm.is_token_member = TRUE)
        OR (p.public_access_level = 'tokenBased' AND p.token_ro = $3)
    )
`, projectId, userId, accessToken).Scan(
		&d.Project.Compiler,
		&d.Project.Epoch,
		&d.Project.ImageName,
		&d.Project.Name,
		&d.Project.OwnerId,
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

func (m *manager) GetProjectAccessForReadAndWriteToken(ctx context.Context, userId sharedTypes.UUID, accessToken AccessToken) (*TokenAccessResult, error) {
	if err := accessToken.ValidateReadAndWrite(); err != nil {
		return nil, err
	}
	return m.getProjectByToken(ctx, userId, accessToken)
}

func (m *manager) GetProjectAccessForReadOnlyToken(ctx context.Context, userId sharedTypes.UUID, accessToken AccessToken) (*TokenAccessResult, error) {
	if err := accessToken.ValidateReadOnly(); err != nil {
		return nil, err
	}
	return m.getProjectByToken(ctx, userId, accessToken)
}

func (m *manager) GetEntries(ctx context.Context, projectId, userId sharedTypes.UUID) (*ForProjectEntries, error) {
	p := &ForProjectEntries{}
	err := m.c.QuerySingle(ctx, `
with
	u := (select User filter .id = <uuid>$1 and not exists .deleted_at)
select Project {
	docs: { resolved_path },
	files: { resolved_path },
}
filter .id = <uuid>$0 and not exists .deleted_at and u in .min_access_ro
`, p, projectId, userId)
	if err != nil {
		return nil, rewriteEdgedbError(err)
	}
	return p, nil
}

func (m *manager) GetProjectMembers(ctx context.Context, projectId sharedTypes.UUID) ([]user.AsProjectMember, error) {
	var p WithInvitedMembers
	err := m.c.QuerySingle(ctx, `
select Project {
	access_ro: {
		email: { email },
		first_name,
		id,
		last_name,
	},
	access_rw: {
		email: { email },
		first_name,
		id,
		last_name,
	},
}
filter .id = <uuid>$0 and not exists .deleted_at
`, &p, projectId)
	if err != nil {
		return nil, rewriteEdgedbError(err)
	}
	return p.GetProjectMembers(), nil
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

func (m *manager) getProjectByToken(ctx context.Context, userId sharedTypes.UUID, accessToken AccessToken) (*TokenAccessResult, error) {
	p := &forTokenAccessCheck{}
	var tokenPrefixRW, tokenRO AccessToken
	if len(accessToken) == lenReadOnly {
		tokenRO = accessToken
	} else {
		tokenPrefixRW = accessToken[:lenReadAndWritePrefix]
	}
	err := m.c.QuerySingle(ctx, `
with
	u := (select User filter .id = <uuid>$0 and not exists .deleted_at)
select Project {
	access_ro: { id } filter User = u,
	access_rw: { id } filter User = u,
	access_token_ro: { id } filter User = u,
	access_token_rw: { id } filter User = u,
	epoch,
	id,
	owner,
	public_access_level,
	tokens: {
		token_ro,
		token_rw,
	},
}
filter
	.public_access_level = 'tokenBased'
 	and not exists .deleted_at
	and (
		.tokens.token_prefix_rw = <str>$1 or .tokens.token_ro = <str>$2
	)
limit 1
`, p, userId, tokenPrefixRW, tokenRO)
	if err != nil {
		return nil, rewriteEdgedbError(err)
	}
	freshAccess, err := p.GetPrivilegeLevelAnonymous(accessToken)
	if err != nil {
		return nil, err
	}
	r := &TokenAccessResult{
		ProjectId: p.Id,
		Epoch:     p.Epoch,
		Fresh:     freshAccess,
	}
	if userId == (sharedTypes.UUID{}) {
		return r, nil
	}
	r.Existing, _ = p.GetPrivilegeLevelAuthenticated(userId)
	return r, nil
}

func (m *manager) GrantReadAndWriteTokenAccess(ctx context.Context, projectId sharedTypes.UUID, epoch int64, userId sharedTypes.UUID) error {
	err := rewriteEdgedbError(m.c.QuerySingle(ctx, `
with
	u := (select User filter .id = <uuid>$2 and not exists .deleted_at)
update Project
filter .id = <uuid>$0 and .epoch = <int64>$1 and not exists .deleted_at
set {
	access_token_rw += u,
	access_token_ro -= u,
	epoch := Project.epoch + 1,
}
`, &IdField{}, projectId, epoch, userId))
	if err != nil && errors.IsNotFoundError(err) {
		return ErrEpochIsNotStable
	}
	return err
}

func (m *manager) GrantReadOnlyTokenAccess(ctx context.Context, projectId sharedTypes.UUID, epoch int64, userId sharedTypes.UUID) error {
	err := rewriteEdgedbError(m.c.QuerySingle(ctx, `
with
	u := (select User filter .id = <uuid>$2 and not exists .deleted_at)
update Project
filter .id = <uuid>$0 and .epoch = <int64>$1
set {
	access_token_ro += u,
	epoch := Project.epoch + 1,
}
`, &IdField{}, projectId, epoch, userId))
	if err != nil && errors.IsNotFoundError(err) {
		return ErrEpochIsNotStable
	}
	return err
}

func (m *manager) RemoveMember(ctx context.Context, projectId sharedTypes.UUID, epoch int64, userId sharedTypes.UUID) error {
	var r []bool
	err := m.c.Query(ctx, `
with
	u := (select User filter .id = <uuid>$0 and not exists .deleted_at),
	p := (
		update Project
		filter
			.id = <uuid>$1
		and .epoch = <int64>$2
 		and not exists .deleted_at
		set {
			epoch := Project.epoch + 1,
			access_ro -= u,
			access_rw -= u,
			access_token_ro -= u,
			access_token_rw -= u,
			archived_by -= u,
			trashed_by -= u,
		}
	),
	tags := (update u.tags set { projects -= p })
select { exists p, exists tags }
`, &r, userId, projectId, epoch)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	if len(r) == 0 || !r[0] {
		return ErrEpochIsNotStable
	}
	return nil
}

type softDeleteResult struct {
	ProjectExists        bool `edgedb:"project_exists"`
	AuthCheck            bool `edgedb:"auth_check"`
	ProjectNotDeletedYet bool `edgedb:"project_not_deleted_yet"`
	ProjectSoftDeleted   bool `edgedb:"project_soft_deleted"`
	AuditLogEntry        bool `edgedb:"audit_log_entry"`
}

func (m *manager) SoftDelete(ctx context.Context, projectId, userId sharedTypes.UUID, ipAddress string) error {
	r := softDeleteResult{}
	err := m.c.QuerySingle(ctx, `
with
	u := (select User filter .id = <uuid>$0 and not exists .deleted_at),
	p := (select Project filter .id = <uuid>$1),
	pWithAuth := (select p filter .owner = u),
	pNotDeletedYet := (select pWithAuth filter not exists .deleted_at),
	pDeleted := (
		update pNotDeletedYet
		set {
			deleted_at := datetime_of_transaction(),
			epoch := pNotDeletedYet.epoch + 1,
		}
	),
	auditLogEntry := (
		insert ProjectAuditLogEntry {
			project := pDeleted,
			initiator := u,
			operation := 'soft-deletion',
			info := <json>{
				ipAddress := <str>$2,
			}
		}
	)
select {
	project_exists := exists p,
	auth_check := exists pWithAuth,
	project_not_deleted_yet := exists pNotDeletedYet,
	project_soft_deleted := exists pDeleted,
	audit_log_entry := exists auditLogEntry,
}
`,
		&r,
		userId, projectId, ipAddress,
	)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	switch {
	case !r.ProjectExists:
		return &errors.NotFoundError{}
	case !r.AuthCheck:
		return &errors.NotAuthorizedError{}
	case !r.ProjectNotDeletedYet:
		return &errors.UnprocessableEntityError{
			Msg: "project already soft deleted",
		}
	default:
		// project soft deleted
		return nil
	}
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
	if n, _ := r.RowsAffected(); n == 0 {
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
	err := m.c.QuerySingle(ctx, `
with
	u := (select User filter .id = <uuid>$1 and not exists .deleted_at),
	p := (select Project filter .id = <uuid>$0),
	pWithAuth := (select p filter .owner = u),
	pDeleted := (select pWithAuth filter exists .deleted_at)
select pDeleted.name
`, &name, projectId, userId)
	if err != nil {
		return "", rewriteEdgedbError(err)
	}
	return name, nil
}

type restoreResult struct {
	ProjectExists       bool `edgedb:"project_exists"`
	AuthCheck           bool `edgedb:"auth_check"`
	ProjectStillDeleted bool `edgedb:"project_still_deleted"`
	ProjectRestored     bool `edgedb:"project_restored"`
}

func (m *manager) Restore(ctx context.Context, projectId, userId sharedTypes.UUID, name Name) error {
	r := restoreResult{}
	err := m.c.QuerySingle(ctx, `
with
	u := (select User filter .id = <uuid>$0 and not exists .deleted_at),
	p := (select Project filter .id = <uuid>$1),
	pWithAuth := (select p filter .owner = u),
	pStillDeleted := (select pWithAuth filter exists .deleted_at),
	pRestored := (
		update pStillDeleted
		set {
			deleted_at := <datetime>{},
			epoch := pStillDeleted.epoch + 1,
			name := <str>$2,
		}
	)
select {
	project_exists := exists p,
	auth_check := exists pWithAuth,
	project_still_deleted := exists pStillDeleted,
	project_updated := exists pRestored,
}
`,
		&r,
		userId, projectId, name,
	)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	switch {
	case !r.ProjectExists:
		return &errors.NotFoundError{}
	case !r.AuthCheck:
		return &errors.NotAuthorizedError{}
	case !r.ProjectStillDeleted:
		return &errors.UnprocessableEntityError{
			Msg: "project not deleted",
		}
	default:
		// project restored
		return nil
	}
}

type createTreeElementResult struct {
	ProjectExists bool                 `edgedb:"project_exists"`
	AuthCheck     bool                 `edgedb:"auth_check"`
	FolderExists  bool                 `edgedb:"folder_exists"`
	ElementId     edgedb.OptionalUUID  `edgedb:"element_id"`
	Linked        bool                 `edgedb:"linked"`
	Version       edgedb.OptionalInt64 `edgedb:"version"`
}

func (m *manager) CreateDoc(ctx context.Context, projectId, userId, folderId sharedTypes.UUID, d *Doc) (sharedTypes.Version, error) {
	result := createTreeElementResult{}
	err := m.c.QuerySingle(ctx, `
with
	p := (select Project filter .id = <uuid>$0 and not exists .deleted_at),
	u := (select User filter .id = <uuid>$1 and not exists .deleted_at),
	pWithAuth := (select p filter u in .min_access_rw),
	parent := (
		select {
			(
				select Folder
				filter
					.id = <uuid>$2
				and .project = pWithAuth
				and not .deleted
			),
			(
				select RootFolder
				filter
					.id = <uuid>$2
				and .project = pWithAuth
			),
		}
		limit 1
	),
	d := (insert Doc {
		name := <str>$3,
		parent := parent,
		project := pWithAuth,
		size := <int64>$4,
		snapshot := <str>$5,
		version := 0,
	}),
	pBumpedVersion := (
		update d.project
		set {
			version := d.project.version + 1,
			last_updated_at := datetime_of_transaction(),
			last_updated_by := u,
		}
	)
select {
	project_exists := (exists p),
	auth_check := (exists pWithAuth),
	folder_exists := (exists parent),
	element_id := d.id,
	version := pBumpedVersion.version,
}
`,
		&result,
		projectId, userId, folderId,
		d.Name, int64(len(d.Snapshot)), d.Snapshot,
	)
	if err != nil {
		if e, ok := err.(edgedb.Error); ok && e.Category(edgedb.ConstraintViolationError) {
			return 0, ErrDuplicateNameInFolder
		}
		return 0, rewriteEdgedbError(err)
	}
	// TODO: read MissingRequiredError details instead of result fields
	switch {
	case !result.ProjectExists:
		return 0, &errors.NotFoundError{}
	case !result.AuthCheck:
		return 0, &errors.NotAuthorizedError{}
	case !result.FolderExists:
		return 0, &errors.UnprocessableEntityError{Msg: "missing folder"}
	default:
		id, _ := result.ElementId.Get()
		d.Id = sharedTypes.UUID(id)
		v, _ := result.Version.Get()
		return sharedTypes.Version(v), nil
	}
}

func (m *manager) CreateFile(ctx context.Context, projectId, userId, folderId sharedTypes.UUID, f *FileRef) (sharedTypes.Version, error) {
	result := createTreeElementResult{}
	err := m.c.QuerySingle(ctx, `
with
	p := (select Project filter .id = <uuid>$0 and not exists .deleted_at),
	u := (select User filter .id = <uuid>$1 and not exists .deleted_at),
	pWithAuth := (select p filter u in .min_access_rw),
	parent := (
		select {
			(
				select Folder
				filter
					.id = <uuid>$2
				and .project = pWithAuth
				and not .deleted
			),
			(
				select RootFolder
				filter
					.id = <uuid>$2
				and .project = pWithAuth
			),
		}
		limit 1
	),
	f := (insert File {
		name := <str>$3,
		parent := parent,
		project := pWithAuth,
		size := <int64>$4,
		hash := <str>$5,
	}),
	linkedFileData := (
		for entry in ({1} if <str>$6 != "" else <int64>{}) union (
			insert LinkedFileData {
				provider := <str>$6,
				source_project_id := <str>$7,
				source_entity_path := <str>$8,
				source_output_file_path := <str>$9,
				url := <str>$10,
				file := f,
			}
		)
	),
	pBumpedVersion := (
		update f.project
		set {
			version := f.project.version + 1,
			last_updated_at := datetime_of_transaction(),
			last_updated_by := u,
		}
	)
select {
	project_exists := (exists p),
	auth_check := (exists pWithAuth),
	folder_exists := (exists parent),
	element_id := f.id,
	linked := exists linkedFileData,
	version := pBumpedVersion.version,
}
`,
		&result,
		projectId, userId, folderId,
		f.Name, f.Size, f.Hash, f.LinkedFileData.Provider,
		f.LinkedFileData.SourceProjectId, f.LinkedFileData.SourceEntityPath,
		f.LinkedFileData.SourceOutputFilePath, f.LinkedFileData.URL,
	)
	if err != nil {
		if e, ok := err.(edgedb.Error); ok && e.Category(edgedb.ConstraintViolationError) {
			return 0, ErrDuplicateNameInFolder
		}
		return 0, rewriteEdgedbError(err)
	}
	// TODO: read MissingRequiredError details instead of result fields
	switch {
	case !result.ProjectExists:
		return 0, &errors.NotFoundError{}
	case !result.AuthCheck:
		return 0, &errors.NotAuthorizedError{}
	case !result.FolderExists:
		return 0, &errors.UnprocessableEntityError{Msg: "missing folder"}
	default:
		id, _ := result.ElementId.Get()
		f.Id = sharedTypes.UUID(id)
		v, _ := result.Version.Get()
		return sharedTypes.Version(v), nil
	}
}

type ForProjectList struct {
	User          user.ProjectListViewCaller
	Tags          tag.Tags
	Projects      List
	Collaborators user.BulkFetched
}

func (m *manager) ListProjects(ctx context.Context, userId sharedTypes.UUID, d *ForProjectList) error {
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
		r, err := m.db.QueryContext(pCtx, `
SELECT archived,
       can_write,
       epoch,
       id,
       is_token_member,
       last_updated_at,
       COALESCE(last_updated_by, '00000000-0000-0000-0000-000000000000'::UUID),
       name,
       owner_id,
       public_access_level,
       trashed
FROM projects p
         INNER JOIN project_members pm ON p.id = pm.project_id
WHERE pm.user_id = $1;
`, userId)
		if err != nil {
			return err
		}
		defer func() { _ = r.Close() }()
		for i := 0; r.Next(); i++ {
			d.Projects = append(d.Projects, ListViewPrivate{})
			err = r.Scan(
				&d.Projects[i].Archived,
				&d.Projects[i].CanWrite,
				&d.Projects[i].Epoch,
				&d.Projects[i].Id,
				&d.Projects[i].IsTokenMember,
				&d.Projects[i].LastUpdatedAt,
				&d.Projects[i].LastUpdatedBy,
				&d.Projects[i].Name,
				&d.Projects[i].OwnerId,
				&d.Projects[i].PublicAccessLevel,
				&d.Projects[i].Trashed,
			)
			if err != nil {
				return err
			}
		}
		return r.Err()
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
