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
	"encoding/json"
	"time"

	"github.com/edgedb/edgedb-go"
	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	spellingTypes "github.com/das7pad/overleaf-go/services/spelling/pkg/types"
)

type Manager interface {
	PrepareProjectCreation(ctx context.Context, p *ForCreation) error
	CreateProjectTree(ctx context.Context, creation *ForCreation) error
	FinalizeProjectCreation(ctx context.Context, p *ForCreation) error
	SoftDelete(ctx context.Context, projectId, userId edgedb.UUID, ipAddress string) error
	HardDelete(ctx context.Context, projectId edgedb.UUID) error
	ProcessSoftDeleted(ctx context.Context, cutOff time.Time, fn func(projectId edgedb.UUID) bool) error
	GetDeletedProjectsName(ctx context.Context, projectId, userId edgedb.UUID) (Name, error)
	Restore(ctx context.Context, projectId, userId edgedb.UUID, name Name) error
	AddFolder(ctx context.Context, projectId, userId, parent edgedb.UUID, f *Folder) (sharedTypes.Version, error)
	DeleteDoc(ctx context.Context, projectId, userId, docId edgedb.UUID) (sharedTypes.Version, error)
	DeleteFile(ctx context.Context, projectId, userId, fileId edgedb.UUID) (sharedTypes.Version, error)
	DeleteFolder(ctx context.Context, projectId, userId, folderId edgedb.UUID) (sharedTypes.Version, error)
	RestoreDoc(ctx context.Context, projectId, userId, docId edgedb.UUID, name sharedTypes.Filename) (*DocWithParent, sharedTypes.Version, error)
	MoveDoc(ctx context.Context, projectId, userId, folderId, docId edgedb.UUID) (sharedTypes.Version, sharedTypes.PathName, error)
	MoveFile(ctx context.Context, projectId, userId, folderId, fileId edgedb.UUID) (sharedTypes.Version, error)
	MoveFolder(ctx context.Context, projectId, userId, targetFolderId, folderId edgedb.UUID) (sharedTypes.Version, []Doc, error)
	RenameDoc(ctx context.Context, projectId, userId edgedb.UUID, d *Doc) (sharedTypes.Version, sharedTypes.DirName, error)
	RenameFile(ctx context.Context, projectId, userId edgedb.UUID, f *FileRef) (sharedTypes.Version, error)
	RenameFolder(ctx context.Context, projectId, userId edgedb.UUID, f *Folder) (sharedTypes.Version, []Doc, error)
	GetAuthorizationDetails(ctx context.Context, projectId, userId edgedb.UUID, token AccessToken) (*AuthorizationDetails, error)
	GetForProjectJWT(ctx context.Context, projectId, userId edgedb.UUID) (*ForAuthorizationDetails, int64, error)
	GetForZip(ctx context.Context, projectId edgedb.UUID, epoch int64) (*ForZip, error)
	ValidateProjectJWTEpochs(ctx context.Context, projectId, userId edgedb.UUID, projectEpoch, userEpoch int64) error
	BumpEpoch(ctx context.Context, projectId edgedb.UUID) error
	GetEpoch(ctx context.Context, projectId edgedb.UUID) (int64, error)
	GetDoc(ctx context.Context, projectId, docId edgedb.UUID) (*Doc, error)
	GetFile(ctx context.Context, projectId, userId edgedb.UUID, accessToken AccessToken, fileId edgedb.UUID) (*FileWithParent, error)
	GetElementHintForOverwrite(ctx context.Context, projectId, userId, folderId edgedb.UUID, name sharedTypes.Filename) (edgedb.UUID, bool, error)
	GetElementByPath(ctx context.Context, projectId, userId edgedb.UUID, path sharedTypes.PathName) (edgedb.UUID, bool, error)
	GetJoinProjectDetails(ctx context.Context, projectId, userId edgedb.UUID, accessToken AccessToken) (*JoinProjectDetails, error)
	GetLoadEditorDetails(ctx context.Context, projectId, userId edgedb.UUID, accessToken AccessToken) (*LoadEditorDetails, error)
	GetProjectRootFolder(ctx context.Context, projectId edgedb.UUID) (*Folder, sharedTypes.Version, error)
	GetProjectWithContent(ctx context.Context, projectId edgedb.UUID) (*Folder, error)
	GetProject(ctx context.Context, projectId edgedb.UUID, target interface{}) error
	GetProjectAccessForReadAndWriteToken(ctx context.Context, userId edgedb.UUID, token AccessToken) (*TokenAccessResult, error)
	GetProjectAccessForReadOnlyToken(ctx context.Context, userId edgedb.UUID, token AccessToken) (*TokenAccessResult, error)
	GetEntries(ctx context.Context, projectId, userId edgedb.UUID) (*ForProjectEntries, error)
	GetProjectMembers(ctx context.Context, projectId edgedb.UUID) ([]user.AsProjectMember, error)
	GrantMemberAccess(ctx context.Context, projectId edgedb.UUID, epoch int64, userId edgedb.UUID, level sharedTypes.PrivilegeLevel) error
	GrantReadAndWriteTokenAccess(ctx context.Context, projectId edgedb.UUID, epoch int64, userId edgedb.UUID) error
	GrantReadOnlyTokenAccess(ctx context.Context, projectId edgedb.UUID, epoch int64, userId edgedb.UUID) error
	PopulateTokens(ctx context.Context, projectId, userId edgedb.UUID) (*Tokens, error)
	GetProjectNames(ctx context.Context, userId edgedb.UUID) (Names, error)
	SetCompiler(ctx context.Context, projectId, userId edgedb.UUID, compiler sharedTypes.Compiler) error
	SetImageName(ctx context.Context, projectId, userId edgedb.UUID, imageName sharedTypes.ImageName) error
	SetSpellCheckLanguage(ctx context.Context, projectId, userId edgedb.UUID, spellCheckLanguage spellingTypes.SpellCheckLanguage) error
	SetRootDoc(ctx context.Context, projectId, userId, rooDocId edgedb.UUID) error
	SetPublicAccessLevel(ctx context.Context, projectId, userId edgedb.UUID, level PublicAccessLevel) error
	ArchiveForUser(ctx context.Context, projectId, userId edgedb.UUID) error
	UnArchiveForUser(ctx context.Context, projectId, userId edgedb.UUID) error
	TrashForUser(ctx context.Context, projectId, userId edgedb.UUID) error
	UnTrashForUser(ctx context.Context, projectId, userId edgedb.UUID) error
	Rename(ctx context.Context, projectId, userId edgedb.UUID, name Name) error
	RemoveMember(ctx context.Context, projectId edgedb.UUID, epoch int64, userId edgedb.UUID) error
	TransferOwnership(ctx context.Context, projectId, previousOwnerId, newOwnerId edgedb.UUID) (*user.WithPublicInfo, *user.WithPublicInfo, Name, error)
	CreateDoc(ctx context.Context, projectId, userId, folderId edgedb.UUID, d *Doc) (sharedTypes.Version, error)
	CreateFile(ctx context.Context, projectId, userId, folderId edgedb.UUID, f *FileRef) (sharedTypes.Version, error)
}

func New(c *edgedb.Client) Manager {
	// TODO: handle deleted projects
	return &manager{
		c: c,
	}
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

type manager struct {
	c *edgedb.Client
}

type docForInsertion struct {
	Name     sharedTypes.Filename `json:"name"`
	Size     int64                `json:"size"`
	Snapshot string               `json:"snapshot"`
}

type fileForInsertion struct {
	Name           sharedTypes.Filename `json:"name"`
	Size           int64                `json:"size"`
	Hash           sharedTypes.Hash     `json:"hash"`
	LinkedFileData []LinkedFileData     `json:"linked_file_data"`
}

type folderForInsertion struct {
	Id    edgedb.UUID        `json:"id"`
	Docs  []docForInsertion  `json:"docs"`
	Files []fileForInsertion `json:"files"`
}

func (m *manager) PrepareProjectCreation(ctx context.Context, p *ForCreation) error {
	ids := make([]edgedb.UUID, 2)
	err := m.c.Query(
		ctx,
		`
with
	owner := (select User { editor_config } filter .id = <uuid>$0),
	provided_lng := <str>$1,
	lng := (
		owner.editor_config.spell_check_language
		if provided_lng = 'inherit' else provided_lng
	),
	p := (insert Project {
		compiler := <str>$2,
		image_name := <str>$3,
		name := <str>$4,
		last_updated_by := owner,
		owner := owner,
		spell_check_language := lng,
	}),
	rf := (insert RootFolder { project := p, path := '' }),
	cr := (insert ChatRoom { project := p }),
select {p.id, rf.id, cr.id}`,
		&ids,
		p.Owner.Id, p.SpellCheckLanguage, p.Compiler, p.ImageName,
		p.Name,
	)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	p.Id = ids[0]
	p.RootFolder.Id = ids[1]
	return nil
}

func (m *manager) CreateProjectTree(ctx context.Context, p *ForCreation) error {
	r := &p.RootFolder
	nFoldersWithContent := 0
	nFoldersWithChildren := 0
	{
		// Collect tree stats for precise allocations
		err := r.WalkFolders(func(f *Folder, _ sharedTypes.DirName) error {
			if len(f.Docs)+len(f.FileRefs) > 0 {
				nFoldersWithContent++
			}
			if len(f.Folders) > 0 {
				nFoldersWithChildren++
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	if nFoldersWithChildren > 0 {
		// Build tree of Folders
		queue := make(chan *Folder, nFoldersWithChildren)
		queue <- &r.Folder
		queueChanges := make(chan int, 10)
		queueChanges <- +1

		eg, pCtx := errgroup.WithContext(ctx)
		for i := 0; i < 5; i++ {
			eg.Go(func() error {
				defer func() {
					for range queue {
						// flush the queue
						queueChanges <- -1
					}
				}()
				for folder := range queue {
					names := make([]string, len(folder.Folders))
					for j, f := range folder.Folders {
						names[j] = string(f.Name)
					}
					ids := make([]IdField, len(folder.Folders))
					err := m.c.QuerySingle(
						pCtx,
						`
with
	project := (select Project filter .id = <uuid>$0),
	parent := (select FolderLike filter .id = <uuid>$1)
for name in array_unpack(<array<str>>$2) union (
	insert Folder {
		project := project,
		parent := parent,
		name := <str>name,
		path := parent.path_for_join ++ <str>name,
	}
)`,
						&ids,
						p.Id, folder.Id, names,
					)
					if err != nil {
						queueChanges <- -1
						return rewriteEdgedbError(err)
					}
					for j := range folder.Folders {
						f := &folder.Folders[j]
						f.Id = ids[j].Id
						if len(f.Folders) > 0 {
							queue <- f
							queueChanges <- +1
						}
					}
					queueChanges <- -1
				}
				return nil
			})
		}

		eg.Go(func() error {
			queueDepth := 0
			for i := range queueChanges {
				queueDepth += i
				if queueDepth == 0 {
					break
				}
			}
			close(queue)
			close(queueChanges)
			return nil
		})

		if err := eg.Wait(); err != nil {
			return err
		}
	}

	nItems := 0
	folders := make([]*folderForInsertion, 0, nFoldersWithContent)
	{
		// Prepare insertion of Docs and Files
		err := r.WalkFolders(func(f *Folder, _ sharedTypes.DirName) error {
			n := len(f.Docs) + len(f.FileRefs)
			if n > 0 {
				nItems += n
				fi := folderForInsertion{
					Id:    f.Id,
					Docs:  make([]docForInsertion, len(f.Docs)),
					Files: make([]fileForInsertion, len(f.FileRefs)),
				}
				for i := range f.Docs {
					fi.Docs[i].Name = f.Docs[i].Name
					fi.Docs[i].Size = f.Docs[i].Size
					fi.Docs[i].Snapshot = f.Docs[i].Snapshot
				}
				for i := range f.FileRefs {
					fi.Files[i].Name = f.FileRefs[i].Name
					fi.Files[i].Size = f.FileRefs[i].Size
					fi.Files[i].Hash = f.FileRefs[i].Hash
					if f.FileRefs[i].LinkedFileData.Provider != "" {
						fi.Files[i].LinkedFileData = []LinkedFileData{
							f.FileRefs[i].LinkedFileData,
						}
					} else {
						fi.Files[i].LinkedFileData = make([]LinkedFileData, 0)
					}
				}
				folders = append(folders, &fi)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	ids := make([]IdField, nItems)
	{
		blob, err := json.Marshal(folders)
		if err != nil {
			return errors.Tag(err, "serialize docs/files")
		}
		// Insert Docs and Files
		err = m.c.Query(ctx, `
with
	project := (select Project filter .id = <uuid>$0)
for folder in json_array_unpack(<json>$1) union (
	with
		f := (select FolderLike filter .id = <uuid>folder['id'])
	select (
		for doc in json_array_unpack(folder['docs']) union (
			insert Doc {
				project := project,
				parent := f,
				name := <str>doc['name'],
				size := <int64>doc['size'],
				snapshot := <str>doc['snapshot'],
				version := 0,
			}
		)
	) union (
		for file in json_array_unpack(folder['files']) union (
			with
				inserted := (
					insert File {
						project := project,
						parent := f,
						name := <str>file['name'],
						size := <int64>file['size'],
						hash := <str>file['hash'],
					}
				)
			select (
				inserted union (
					for entry in json_array_unpack(file['linked_file_data']) union (
						select (
							insert LinkedFileData {
								provider := <str>entry['provider'],
								source_project_id := <str>json_get(
									entry, 'source_project_id') ?? '',
								source_entity_path := <str>json_get(
									entry, 'source_entity_path') ?? '',
								source_output_file_path := <str>json_get(
									entry, 'source_output_file_path') ?? '',
								url := <str>json_get(entry, 'url') ?? '',

								file := inserted
							}
						).file
					)
				)
			) limit 1
		)
	)
)
`,
			&ids,
			p.Id, blob,
		)
		if err != nil {
			return rewriteEdgedbError(err)
		}
	}

	{
		// Back-fill ids
		i := 0
		err := r.WalkFolders(func(f *Folder, _ sharedTypes.DirName) error {
			for j := range f.Docs {
				f.Docs[j].Id = ids[i].Id
				i++
			}
			for j := range f.FileRefs {
				f.FileRefs[j].Id = ids[i].Id
				i++
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *manager) FinalizeProjectCreation(ctx context.Context, p *ForCreation) error {
	err := m.c.QuerySingle(ctx, `
update Project
filter .id = <uuid>$0
set {
	name := <str>$1,
	root_doc := (
		select Doc
		filter .id = <uuid>$2 and .project.id = <uuid>$0
	)
}
`,
		&IdField{},
		p.Id, p.Name, p.RootDoc.Id,
	)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	return nil
}

func (m *manager) PopulateTokens(ctx context.Context, projectId, userId edgedb.UUID) (*Tokens, error) {
	allErrors := &errors.MergedError{}
	for i := 0; i < 10; i++ {
		tokens, err := generateTokens()
		if err != nil {
			allErrors.Add(err)
			continue
		}
		ids := make([]IdField, 0, 3)
		err = m.c.Query(ctx, `
select (
	select Project filter .id = <uuid>$0
) union (
	select Project filter .id = <uuid>$0 and .owner.id = <uuid>$1
) union (
	update Project
	filter
		.id = <uuid>$0
	and .owner.id = <uuid>$1
	and (.tokens.token_ro ?? '') = ''
	set {
		tokens := (insert Tokens {
			token_ro := <str>$2,
			token_rw := <str>$3,
			token_prefix_rw := <str>$4,
		}),
	}
)
`,
			&ids,
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
		if err = checkAuthExistsGuard(ids); err != nil {
			return nil, err
		}
		if len(ids) == 3 {
			return tokens, nil
		}
		// tokens are already populated
		return nil, nil
	}
	return nil, errors.Tag(allErrors, "bad random source")
}

func (m *manager) SetCompiler(ctx context.Context, projectId, userId edgedb.UUID, compiler sharedTypes.Compiler) error {
	ids := make([]IdField, 0)
	err := m.c.Query(ctx, `
select (
	select Project filter .id = <uuid>$0
) union (
	update Project
	filter .id = <uuid>$0
	and (select User filter .id = <uuid>$1) in .min_access_rw
	set {
		compiler := <str>$2
	}
)`, &ids, projectId, userId, string(compiler))
	if err != nil {
		return rewriteEdgedbError(err)
	}
	return checkAuthExistsGuard(ids)
}

func (m *manager) SetImageName(ctx context.Context, projectId, userId edgedb.UUID, imageName sharedTypes.ImageName) error {
	ids := make([]IdField, 0)
	err := m.c.Query(ctx, `
select (
	select Project filter .id = <uuid>$0
) union (
	update Project
	filter .id = <uuid>$0
	and (select User filter .id = <uuid>$1) in .min_access_rw
	set {
		image_name := <str>$2
	}
)`, &ids, projectId, userId, string(imageName))
	if err != nil {
		return rewriteEdgedbError(err)
	}
	return checkAuthExistsGuard(ids)
}

func (m *manager) SetSpellCheckLanguage(ctx context.Context, projectId, userId edgedb.UUID, spellCheckLanguage spellingTypes.SpellCheckLanguage) error {
	ids := make([]IdField, 0)
	err := m.c.Query(ctx, `
select (
	select Project filter .id = <uuid>$0
) union (
	update Project
	filter .id = <uuid>$0
	and (select User filter .id = <uuid>$1) in .min_access_rw
	set {
		spell_check_language := <str>$2
	}
)`, &ids, projectId, userId, string(spellCheckLanguage))
	if err != nil {
		return rewriteEdgedbError(err)
	}
	return checkAuthExistsGuard(ids)
}

type setRootDocResult struct {
	ProjectExists   bool `edgedb:"project_exists"`
	AuthCheck       bool `edgedb:"auth_check"`
	DocExists       bool `edgedb:"doc_exists"`
	RootDocEligible bool `edgedb:"root_doc_eligible"`
	ProjectUpdated  bool `edgedb:"project_updated"`
}

func (m *manager) SetRootDoc(ctx context.Context, projectId, userId, rootDocId edgedb.UUID) error {
	r := setRootDocResult{}
	err := m.c.QuerySingle(ctx, `
with
	p := (select Project filter .id = <uuid>$0),
	u := (select User filter .id = <uuid>$2),
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

func (m *manager) SetPublicAccessLevel(ctx context.Context, projectId, userId edgedb.UUID, publicAccessLevel PublicAccessLevel) error {
	ids := make([]IdField, 0)
	err := m.c.Query(ctx, `
select (
	select Project filter .id = <uuid>$0
) union (
	update Project
	filter .id = <uuid>$0 and .owner.id = <uuid>$1
	set {
		public_access_level := <str>$2
	}
)`, &ids, projectId, userId, string(publicAccessLevel))
	if err != nil {
		return rewriteEdgedbError(err)
	}
	return checkAuthExistsGuard(ids)
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

func (m *manager) TransferOwnership(ctx context.Context, projectId, previousOwnerId, newOwnerId edgedb.UUID) (*user.WithPublicInfo, *user.WithPublicInfo, Name, error) {
	r := transferOwnershipResult{}
	err := m.c.QuerySingle(ctx, `
with
	previousOwner := (select User filter .id = <uuid>$1),
	newOwner := (select User filter .id = <uuid>$2),
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

func checkAuthExistsGuard(ids []IdField) error {
	switch len(ids) {
	case 0:
		return &errors.NotFoundError{}
	case 1:
		return &errors.NotAuthorizedError{}
	default:
		// 2
		return nil
	}
}

func (m *manager) Rename(ctx context.Context, projectId, userId edgedb.UUID, name Name) error {
	ids := make([]IdField, 0)
	err := m.c.Query(ctx, `
select (
	select Project filter .id = <uuid>$0
) union (
	update Project
	filter .id = <uuid>$0 and .owner.id = <uuid>$1
	set {
		name := <str>$2,
	}
)`,
		&ids,
		projectId, userId, string(name),
	)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	return checkAuthExistsGuard(ids)
}

type addTreeElementResult struct {
	ProjectExists  bool                `edgedb:"project_exists"`
	ProjectVersion sharedTypes.Version `edgedb:"project_version"`
	ElementId      edgedb.UUID         `edgedb:"element_id"`
}

func (m *manager) AddFolder(ctx context.Context, projectId, userId, parent edgedb.UUID, f *Folder) (sharedTypes.Version, error) {
	r := addTreeElementResult{}
	err := m.c.QuerySingle(ctx, `
with
	p := (select Project filter .id = <uuid>$0),
	u := (select User filter .id = <uuid>$1),
	pWithAuth := (select p filter u in .min_access_rw),
	f := (select FolderLike filter .id = <uuid>$2 and .project = pWithAuth),
	newFolder := (
		insert Folder {
			project := pWithAuth,
			parent := f,
			name := <str>$3,
			path := f.path_for_join ++ <str>$3
		}
	),
	pBumpedVersion := (
		update newFolder.project
		set {
			version := newFolder.project.version + 1,
			last_updated_at := datetime_of_transaction(),
			last_updated_by := u,
		}
	)
select {
	project_exists := exists p,
	project_version := pBumpedVersion.version ?? 0,
	element_id := newFolder.id,
}
`, &r, projectId, userId, parent, f.Name)
	if err != nil {
		return 0, rewriteEdgedbError(err)
	}
	// TODO: convert missing property error into 422
	if !r.ProjectExists {
		return 0, &errors.UnprocessableEntityError{
			Msg: "project does not exist",
		}
	}
	f.Id = r.ElementId
	return r.ProjectVersion, nil
}

type deletedTreeElementResult struct {
	ProjectExists  bool                `edgedb:"project_exists"`
	HasWriteAccess bool                `edgedb:"has_write_access"`
	ElementExists  bool                `edgedb:"element_exists"`
	ProjectVersion sharedTypes.Version `edgedb:"project_version"`
}

func (m *manager) DeleteDoc(ctx context.Context, projectId, userId, docId edgedb.UUID) (sharedTypes.Version, error) {
	r := deletedTreeElementResult{}
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
	has_write_access := exists pWithAuth,
	element_exists := exists d,
	project_version := pBumpedVersion.version ?? 0,
}
`, &r, projectId, userId, docId)
	if err != nil {
		return 0, rewriteEdgedbError(err)
	}
	switch {
	case !r.ProjectExists:
		return 0, &errors.UnprocessableEntityError{
			Msg: "project does not exist",
		}
	case !r.HasWriteAccess:
		return 0, &errors.NotAuthorizedError{}
	case !r.ElementExists:
		return 0, &errors.UnprocessableEntityError{
			Msg: "doc does not exist",
		}
	}
	return r.ProjectVersion, nil
}

func (m *manager) DeleteFile(ctx context.Context, projectId, userId, fileId edgedb.UUID) (sharedTypes.Version, error) {
	r := deletedTreeElementResult{}
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
	has_write_access := exists pWithAuth,
	element_exists := exists f,
	project_version := pBumpedVersion.version ?? 0,
}
`, &r, projectId, userId, fileId)
	if err != nil {
		return 0, rewriteEdgedbError(err)
	}
	switch {
	case !r.ProjectExists:
		return 0, &errors.UnprocessableEntityError{
			Msg: "project does not exist",
		}
	case !r.HasWriteAccess:
		return 0, &errors.NotAuthorizedError{}
	case !r.ElementExists:
		return 0, &errors.UnprocessableEntityError{
			Msg: "doc does not exist",
		}
	}
	return r.ProjectVersion, nil
}

type deletedFolderResult struct {
	deletedTreeElementResult `edgedb:"$inline"`
	DeletedChildren          bool `edgedb:"deleted_children"`
}

func (m *manager) DeleteFolder(ctx context.Context, projectId, userId, folderId edgedb.UUID) (sharedTypes.Version, error) {
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
	has_write_access := exists pWithAuth,
	element_exists := exists f,
	deleted_children := exists deletedItems,
	project_version := pBumpedVersion.version ?? 0,
}
`, &r, projectId, userId, folderId)
	if err != nil {
		return 0, rewriteEdgedbError(err)
	}
	switch {
	case !r.ProjectExists:
		return 0, &errors.UnprocessableEntityError{
			Msg: "project does not exist",
		}
	case !r.HasWriteAccess:
		return 0, &errors.NotAuthorizedError{}
	case !r.ElementExists:
		return 0, &errors.UnprocessableEntityError{
			Msg: "doc does not exist",
		}
	}
	return r.ProjectVersion, nil
}

type moveTreeElementResult struct {
	ProjectExists  bool                 `edgedb:"project_exists"`
	HasWriteAccess bool                 `edgedb:"has_write_access"`
	ElementExists  bool                 `edgedb:"element_exists"`
	NewPath        sharedTypes.PathName `edgedb:"new_path"`
	ProjectVersion sharedTypes.Version  `edgedb:"project_version"`
}

func (m *manager) MoveDoc(ctx context.Context, projectId, userId, folderId, docId edgedb.UUID) (sharedTypes.Version, sharedTypes.PathName, error) {
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
	has_write_access := exists pWithAuth,
	element_exists := exists d,
	new_path := updatedDoc.resolved_path ?? '',
	project_version := pBumpedVersion.version ?? 0,
}
`, &r, projectId, userId, folderId, docId)
	if err != nil {
		return 0, "", rewriteEdgedbError(err)
	}
	switch {
	case !r.ProjectExists:
		return 0, "", &errors.UnprocessableEntityError{
			Msg: "project does not exist",
		}
	case !r.HasWriteAccess:
		return 0, "", &errors.NotAuthorizedError{}
	case !r.ElementExists:
		return 0, "", &errors.UnprocessableEntityError{
			Msg: "doc does not exist",
		}
	}
	return r.ProjectVersion, r.NewPath, nil
}

func (m *manager) MoveFile(ctx context.Context, projectId, userId, folderId, fileId edgedb.UUID) (sharedTypes.Version, error) {
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
	has_write_access := exists pWithAuth,
	element_exists := exists f,
	project_version := pBumpedVersion.version ?? 0,
}
`, &r, projectId, userId, folderId, fileId)
	if err != nil {
		return 0, rewriteEdgedbError(err)
	}
	switch {
	case !r.ProjectExists:
		return 0, &errors.UnprocessableEntityError{
			Msg: "project does not exist",
		}
	case !r.HasWriteAccess:
		return 0, &errors.NotAuthorizedError{}
	case !r.ElementExists:
		return 0, &errors.UnprocessableEntityError{
			Msg: "file does not exist",
		}
	}
	return r.ProjectVersion, nil
}

type moveFolderResult struct {
	moveTreeElementResult `edgedb:"$inline"`
	DocsField             `edgedb:"$inline"`
	TargetExists          bool `edgedb:"target_exists"`
	TargetLoopCheck       bool `edgedb:"target_loop_check"`
}

func (m *manager) MoveFolder(ctx context.Context, projectId, userId, targetFolderId, folderId edgedb.UUID) (sharedTypes.Version, []Doc, error) {
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
	has_write_access := exists pWithAuth,
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
	switch {
	case !r.ProjectExists:
		return 0, nil, &errors.UnprocessableEntityError{
			Msg: "project does not exist",
		}
	case !r.HasWriteAccess:
		return 0, nil, &errors.NotAuthorizedError{}
	case !r.ElementExists:
		return 0, nil, &errors.UnprocessableEntityError{
			Msg: "file does not exist",
		}
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
	ProjectExists  bool                `edgedb:"project_exists"`
	HasWriteAccess bool                `edgedb:"has_write_access"`
	ElementExists  bool                `edgedb:"element_exists"`
	ParentPath     sharedTypes.DirName `edgedb:"parent_path"`
	ProjectVersion sharedTypes.Version `edgedb:"project_version"`
}

func (m *manager) RenameDoc(ctx context.Context, projectId, userId edgedb.UUID, d *Doc) (sharedTypes.Version, sharedTypes.DirName, error) {
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
	has_write_access := exists pWithAuth,
	element_exists := exists d,
	parent_path := updatedDoc.parent.path ?? '',
	project_version := pBumpedVersion.version ?? 0,
}
`, &r, projectId, userId, d.Id, d.Name)
	if err != nil {
		return 0, "", rewriteEdgedbError(err)
	}
	switch {
	case !r.ProjectExists:
		return 0, "", &errors.UnprocessableEntityError{
			Msg: "project does not exist",
		}
	case !r.HasWriteAccess:
		return 0, "", &errors.NotAuthorizedError{}
	case !r.ElementExists:
		return 0, "", &errors.UnprocessableEntityError{Msg: "doc does not exist"}
	}
	return r.ProjectVersion, r.ParentPath, nil
}

func (m *manager) RenameFile(ctx context.Context, projectId, userId edgedb.UUID, f *FileRef) (sharedTypes.Version, error) {
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
	has_write_access := exists pWithAuth,
	element_exists := exists f,
	project_version := pBumpedVersion.version ?? 0,
}
`, &r, projectId, userId, f.Id, f.Name)
	if err != nil {
		return 0, rewriteEdgedbError(err)
	}
	switch {
	case !r.ProjectExists:
		return 0, &errors.UnprocessableEntityError{
			Msg: "project does not exist",
		}
	case !r.HasWriteAccess:
		return 0, &errors.NotAuthorizedError{}
	case !r.ElementExists:
		return 0, &errors.UnprocessableEntityError{Msg: "file does not exist"}
	}
	return r.ProjectVersion, nil
}

type renameFolderResult struct {
	renameTreeElementResult `edgedb:"$inline"`
	DocsField               `edgedb:"$inline"`
}

func (m *manager) RenameFolder(ctx context.Context, projectId, userId edgedb.UUID, f *Folder) (sharedTypes.Version, []Doc, error) {
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
	has_write_access := exists pWithAuth,
	element_exists := exists f,
	parent_path := f.parent.path ?? '',
	project_version := pBumpedVersion.version ?? 0,
	docs := (select updatedFolders.docs { id, resolved_path }),
}
`, &r, projectId, userId, f.Id, f.Name)
	if err != nil {
		return 0, nil, rewriteEdgedbError(err)
	}
	switch {
	case !r.ProjectExists:
		return 0, nil, &errors.UnprocessableEntityError{
			Msg: "project does not exist",
		}
	case !r.HasWriteAccess:
		return 0, nil, &errors.NotAuthorizedError{}
	case !r.ElementExists:
		return 0, nil, &errors.UnprocessableEntityError{
			Msg: "folder does not exist",
		}
	}
	return r.ProjectVersion, r.Docs, nil
}

func (m *manager) ArchiveForUser(ctx context.Context, projectId, userId edgedb.UUID) error {
	ids := make([]IdField, 0)
	err := m.c.Query(ctx, `
select (
	select Project filter .id = <uuid>$0
) union (
	with u := (select User filter .id = <uuid>$1)
	update Project
	filter .id = <uuid>$0 and u in .min_access_ro
	set {
		archived_by := distinct (Project.archived_by union {u}),
		trashed_by -= u
	}
)`, &ids, projectId, userId)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	return checkAuthExistsGuard(ids)
}

func (m *manager) UnArchiveForUser(ctx context.Context, projectId, userId edgedb.UUID) error {
	ids := make([]IdField, 0)
	err := m.c.Query(ctx, `
select (
	select Project filter .id = <uuid>$0
) union (
	with u := (select User filter .id = <uuid>$1)
	update Project
	filter .id = <uuid>$0 and u in .min_access_ro
	set {
		archived_by -= u
	}
)`, &ids, projectId, userId)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	return checkAuthExistsGuard(ids)
}

func (m *manager) TrashForUser(ctx context.Context, projectId, userId edgedb.UUID) error {
	ids := make([]IdField, 0)
	err := m.c.Query(ctx, `
select (
	select Project filter .id = <uuid>$0
) union (
	with u := (select User filter .id = <uuid>$1)
	update Project
	filter .id = <uuid>$0 and u in .min_access_ro
	set {
		trashed_by := distinct (Project.trashed_by union {u}),
		archived_by -= u
	}
)`, &ids, projectId, userId)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	return checkAuthExistsGuard(ids)
}

func (m *manager) UnTrashForUser(ctx context.Context, projectId, userId edgedb.UUID) error {
	ids := make([]IdField, 0)
	err := m.c.Query(ctx, `
select (
	select Project filter .id = <uuid>$0
) union (
	with u := (select User filter .id = <uuid>$1)
	update Project
	filter .id = <uuid>$0 and u in .min_access_ro
	set {
		trashed_by -= u
	}
)`, &ids, projectId, userId)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	return checkAuthExistsGuard(ids)
}

var ErrEpochIsNotStable = errors.New("epoch is not stable")

type forGetProjectNames struct {
	Projects []NameField `edgedb:"projects"`
}

func (m *manager) GetProjectNames(ctx context.Context, userId edgedb.UUID) (Names, error) {
	u := &forGetProjectNames{}
	err := m.c.QuerySingle(ctx, `
select User {
	projects: { name },
}
filter .id = <uuid>$0
`, u, userId)
	if err != nil {
		return nil, rewriteEdgedbError(err)
	}
	names := make(Names, len(u.Projects))
	for i, project := range u.Projects {
		names[i] = project.Name
	}
	return names, nil
}

func (m *manager) GetAuthorizationDetails(ctx context.Context, projectId, userId edgedb.UUID, token AccessToken) (*AuthorizationDetails, error) {
	p := &ForAuthorizationDetails{}
	err := m.c.QuerySingle(ctx, `
with
	u := (select User filter .id = <uuid>$1)
select Project {
	access_ro: { id } filter User = u,
	access_rw: { id } filter User = u,
	access_token_ro: { id } filter User = u,
	access_token_rw: { id } filter User = u,
	epoch,
	owner,
	public_access_level,
	tokens: {
		token_ro,
		token_rw,
	},
}
filter .id = <uuid>$0
`, p, projectId, userId)
	if err != nil {
		return nil, rewriteEdgedbError(err)
	}
	return p.GetPrivilegeLevel(userId, token)
}

type getForProjectJWTResult struct {
	UserEpoch edgedb.OptionalInt64    `edgedb:"user_epoch"`
	Project   ForAuthorizationDetails `edgedb:"project"`
}

func (m *manager) GetForProjectJWT(ctx context.Context, projectId, userId edgedb.UUID) (*ForAuthorizationDetails, int64, error) {
	r := getForProjectJWTResult{}
	err := m.c.QuerySingle(ctx, `
with
	p := (select Project filter .id = <uuid>$0),
	u := (select User filter .id = <uuid>$1)
select {
	user_epoch := (select u { epoch }).epoch,
	project := (select p {
		access_ro: { id } filter User = u,
		access_rw: { id } filter User = u,
		access_token_ro: { id } filter User = u,
		access_token_rw: { id } filter User = u,
		epoch,
		owner: {
			id,
			features: {
				compile_group,
				compile_timeout,
			},
		},
		public_access_level,
		tokens: {
			token_ro,
			token_rw,
		},
	})
}
`, &r, projectId, userId)
	if err != nil {
		return nil, 0, rewriteEdgedbError(err)
	}
	if r.Project.Owner.Id == (edgedb.UUID{}) {
		return nil, 0, &errors.NotFoundError{}
	}
	userEpoch, _ := r.UserEpoch.Get()
	return &r.Project, userEpoch, nil
}

func (m *manager) ValidateProjectJWTEpochs(ctx context.Context, projectId, userId edgedb.UUID, projectEpoch, userEpoch int64) error {
	if userId == (edgedb.UUID{}) {
		ok := false
		err := m.c.QuerySingle(ctx, `
select { exists (select Project filter .id = <uuid>$0 and .epoch = <int64>$1) }
`, &ok, projectId, projectEpoch)
		if err != nil {
			return rewriteEdgedbError(err)
		}
		if !ok {
			return &errors.UnauthorizedError{Reason: "epoch mismatch: project"}
		}
		return nil
	}

	ok := make([]bool, 2, 2)
	err := m.c.Query(ctx, `
select {
	exists (select Project filter .id = <uuid>$0 and .epoch = <int64>$1),
	exists (select User filter .id = <uuid>$2 and .epoch = <int64>$3),
}
`, &ok, projectId, projectEpoch, userId, userEpoch)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	if !ok[0] {
		return &errors.UnauthorizedError{Reason: "epoch mismatch: project"}
	}
	if !ok[1] {
		return &errors.UnauthorizedError{Reason: "epoch mismatch: user"}
	}
	return nil
}

func (m *manager) BumpEpoch(ctx context.Context, projectId edgedb.UUID) error {
	err := m.c.QuerySingle(ctx, `
update Project
filter .id = <uuid>$0
set {
	epoch := Project.epoch + 1,
}
`, &IdField{}, projectId)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	return nil
}

func (m *manager) GetEpoch(ctx context.Context, projectId edgedb.UUID) (int64, error) {
	p := &EpochField{}
	err := m.c.QuerySingle(ctx, `
select Project { epoch } filter .id = <uuid>$0
`, p, projectId)
	if err != nil {
		return 0, rewriteEdgedbError(err)
	}
	return p.Epoch, err
}

func (m *manager) GetDoc(ctx context.Context, projectId, docId edgedb.UUID) (*Doc, error) {
	d := &Doc{}
	err := m.c.QuerySingle(ctx, `
select Doc {
	id,
	name,
	resolved_path,
	size,
	snapshot,
	version,
}
filter
	.id = <uuid>$0
and not .deleted
and .project.id = <uuid>$1
`, d, docId, projectId)
	if err != nil {
		err = rewriteEdgedbError(err)
		if errors.IsNotFoundError(err) {
			return nil, &errors.ErrorDocNotFound{}
		}
		return nil, err
	}
	return d, nil
}

type restoreDocResult struct {
	ProjectExists bool `edgedb:"project_exists"`
	AuthCheck     bool `edgedb:"auth_check"`
	DocExists     bool `edgedb:"doc_exists"`
	DocDeleted    bool `edgedb:"doc_deleted"`
	Doc           struct {
		edgedb.Optional
		DocWithParent `edgedb:"$inline"`
	} `edgedb:"doc"`
	ProjectVersion sharedTypes.Version `edgedb:"project_version"`
}

func (m *manager) RestoreDoc(ctx context.Context, projectId, userId, docId edgedb.UUID, name sharedTypes.Filename) (*DocWithParent, sharedTypes.Version, error) {
	r := restoreDocResult{}
	err := m.c.QuerySingle(ctx, `
with
	u := (select User filter .id = <uuid>$1),
	p := (select Project filter .id = <uuid>$0),
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
	doc_exists := exists d,
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
	switch {
	case !r.ProjectExists:
		return nil, 0, &errors.NotFoundError{}
	case !r.AuthCheck:
		return nil, 0, &errors.NotAuthorizedError{}
	case !r.DocExists:
		return nil, 0, &errors.ErrorDocNotFound{}
	case !r.DocDeleted:
		return nil, 0, &errors.UnprocessableEntityError{Msg: "doc not deleted"}
	}
	return &r.Doc.DocWithParent, r.ProjectVersion, nil
}

func (m *manager) GetFile(ctx context.Context, projectId, userId edgedb.UUID, accessToken AccessToken, fileId edgedb.UUID) (*FileWithParent, error) {
	f := &FileWithParent{}
	err := m.c.QuerySingle(ctx, `
with
	u := (select User filter .id = <uuid>$2),
	p := (select Project filter .id = <uuid>$1),
	pWithAuth := (
		select {
			(select p filter u in .min_access_ro),
			(
				select p
				filter
					.public_access_level = 'tokenBased'
				and (.tokens.token_ro ?? '') = <str>$3
			),
		}
		limit 1
	)
select File {
	id,
	linked_file_data: {
		provider,
		source_project_id,
		source_entity_path,
		source_output_file_path,
		url,
	},
	name,
	parent,
	size,
}
filter
	.id = <uuid>$0
and not .deleted
and .project = pWithAuth
`, f, fileId, projectId, userId, accessToken)
	if err != nil {
		return nil, rewriteEdgedbError(err)
	}
	return f, nil
}

type getDocOrFileResult struct {
	ProjectExists bool                `edgedb:"project_exists"`
	AuthCheck     bool                `edgedb:"auth_check"`
	ParentExists  bool                `edgedb:"parent_exists"`
	IsDoc         bool                `edgedb:"is_doc"`
	IsFolder      bool                `edgedb:"is_folder"`
	ElementId     edgedb.OptionalUUID `edgedb:"element_id"`
}

func (m *manager) GetElementHintForOverwrite(ctx context.Context, projectId, userId, folderId edgedb.UUID, name sharedTypes.Filename) (edgedb.UUID, bool, error) {
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
		return edgedb.UUID{}, false, rewriteEdgedbError(err)
	}
	id, _ := r.ElementId.Get()
	switch {
	case !r.ProjectExists:
		return edgedb.UUID{}, false, &errors.UnprocessableEntityError{
			Msg: "project does not exist",
		}
	case !r.AuthCheck:
		return edgedb.UUID{}, false, &errors.NotAuthorizedError{}
	case !r.ParentExists:
		return edgedb.UUID{}, false, &errors.UnprocessableEntityError{
			Msg: "parent folder does not exist",
		}
	case r.IsFolder:
		return edgedb.UUID{}, false, &errors.UnprocessableEntityError{
			Msg: "element is a folder",
		}
	}
	return id, r.IsDoc, nil
}

func (m *manager) GetElementByPath(ctx context.Context, projectId, userId edgedb.UUID, path sharedTypes.PathName) (edgedb.UUID, bool, error) {
	r := getDocOrFileResult{}
	err := m.c.QuerySingle(ctx, `
with
	p := (select Project filter .id = <uuid>$0),
	u := (select User filter .id = <uuid>$1),
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
		return edgedb.UUID{}, false, rewriteEdgedbError(err)
	}
	id, exists := r.ElementId.Get()
	switch {
	case !r.ProjectExists:
		return edgedb.UUID{}, false, &errors.UnprocessableEntityError{
			Msg: "project does not exist",
		}
	case !r.AuthCheck:
		return edgedb.UUID{}, false, &errors.NotAuthorizedError{}
	case !exists:
		return edgedb.UUID{}, false, &errors.NotFoundError{}
	}
	return id, r.IsDoc, nil
}

func (m *manager) GetProjectRootFolder(ctx context.Context, projectId edgedb.UUID) (*Folder, sharedTypes.Version, error) {
	project := &ForTree{
		RootFolderField: RootFolderField{
			RootFolder: RootFolder{
				Folder: NewFolder(""),
			},
		},
	}
	err := m.c.QuerySingle(
		ctx,
		`
select
	Project {
		version,
		root_folder: {
			id,
			folders,
			docs: { id, name },
			files: { id, name },
		},
		folders: {
			id,
			name,
			folders,
			docs: { id, name },
			files: { id, name },
		},
	}
filter .id = <uuid>$0`,
		project,
		projectId,
	)
	if err != nil {
		return nil, 0, rewriteEdgedbError(err)
	}
	return project.GetRootFolder(), project.Version, nil
}

func (m *manager) GetProjectWithContent(ctx context.Context, projectId edgedb.UUID) (*Folder, error) {
	project := &ForTree{}
	err := m.c.QuerySingle(
		ctx,
		`
select
	Project {
		root_folder: {
			id,
			folders,
			docs: { id, name, snapshot, version },
			files: { id, name, created_at },
		},
		folders: {
			id,
			name,
			folders,
			docs: { id, name, snapshot, version },
			files: { id, name, created_at },
		},
	}
filter .id = <uuid>$0`,
		project,
		projectId,
	)
	if err != nil {
		return nil, rewriteEdgedbError(err)
	}
	return project.GetRootFolder(), nil
}

func (m *manager) GetForZip(ctx context.Context, projectId edgedb.UUID, epoch int64) (*ForZip, error) {
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
filter .id = <uuid>$0 and .epoch = <int64>$1
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

func (m *manager) GetJoinProjectDetails(ctx context.Context, projectId, userId edgedb.UUID, accessToken AccessToken) (*JoinProjectDetails, error) {
	details := &JoinProjectDetails{
		Project: JoinProjectViewPrivate{
			ForTree: ForTree{
				RootFolderField: RootFolderField{
					RootFolder: RootFolder{
						Folder: NewFolder(""),
					},
				},
			},
		},
	}
	if userId != (edgedb.UUID{}) {
		accessToken = "-"
	}
	err := m.c.QuerySingle(ctx, `
with
	u := (select User filter .id = <uuid>$1),
	p := (select Project filter .id = <uuid>$0),
	pWithAuth := (
		select {
			(select p filter u in .min_access_ro),
			(
				select p
				filter
					.public_access_level = 'tokenBased'
				and (.tokens.token_ro ?? '') = <str>$2
			),
		}
		limit 1
	)
select {
	project_exists := (exists p),
	project := (select pWithAuth {
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
		access_token_ro: { id } filter User = u,
		access_token_rw: { id } filter User = u,
		compiler,
		deleted_docs: { id, name },
		epoch,
		folders: {
			id,
			docs: {
				id,
				name,
			},
			files: {
				created_at,
				id,
				linked_file_data: {
					provider,
					source_project_id,
					source_entity_path,
					source_output_file_path,
					url,
				},
				name,
				size,
			},
			folders,
			name,
		},
		id,
		image_name,
		invites: {
			created_at,
			email,
			expires_at,
			id,
			privilege_level,
			sending_user,
		},
		name,
		owner: {
			email: { email },
			first_name,
			id,
			last_name,
			features: {
				compile_group,
				compile_timeout,
			},
		},
		public_access_level,
		root_doc,
		root_folder: {
			id,
			docs: {
				id,
				name,
			},
			files: {
				created_at,
				id,
				linked_file_data: {
					provider,
					source_project_id,
					source_entity_path,
					source_output_file_path,
					url,
				},
				name,
				size,
			},
			folders,
		},
		spell_check_language,
		tokens: {
			token_ro,
			token_rw,
		},
		version,
	}),
}
`, details, projectId, userId, accessToken)
	if err != nil {
		return nil, rewriteEdgedbError(err)
	}
	if !details.ProjectExists {
		return nil, &errors.NotFoundError{}
	}
	if details.Project.Id != projectId {
		return nil, &errors.NotAuthorizedError{}
	}
	return details, nil
}

func (m *manager) GetLoadEditorDetails(ctx context.Context, projectId, userId edgedb.UUID, accessToken AccessToken) (*LoadEditorDetails, error) {
	details := &LoadEditorDetails{}
	if userId != (edgedb.UUID{}) {
		accessToken = "-"
	}
	err := m.c.QuerySingle(ctx, `
with
	u := (select User filter .id = <uuid>$1),
	p := (select Project filter .id = <uuid>$0),
	pWithAuth := (
		select {
			(select p filter u in .min_access_ro),
			(
				select p
				filter
					.public_access_level = 'tokenBased'
				and (.tokens.token_ro ?? '') = <str>$2
			),
		}
		limit 1
	),
	pBumpedLastOpened := (update pWithAuth set {
		last_opened := datetime_of_transaction(),
	})
select {
	user := (select u {
		editor_config: {
			auto_complete,
			auto_pair_delimiters,
			font_family,
			font_size,
			line_height,
			mode,
			overall_theme,
			pdf_viewer,
			syntax_validation,
			spell_check_language,
			theme,
		},
		email: { email },
		epoch,
		first_name,
		id,
		last_name,
	}),
	project_exists := (exists p),
	project := (select pBumpedLastOpened {
		access_ro: { id } filter User = u,
		access_rw: { id } filter User = u,
		access_token_ro: { id } filter User = u,
		access_token_rw: { id } filter User = u,
		compiler,
		epoch,
		id,
		image_name,
		name,
		owner: {
			email: { email },
			first_name,
			id,
			last_name,
			features: {
				compile_group,
				compile_timeout,
			},
		},
		public_access_level,
		root_doc: {
			id,
			resolved_path,
		},
		tokens: {
			token_ro,
			token_rw,
		},
		version,
	}),
}
`, details, projectId, userId, accessToken)
	if err != nil {
		return nil, rewriteEdgedbError(err)
	}
	if !details.ProjectExists {
		return nil, &errors.NotFoundError{}
	}
	if details.Project.Id != projectId {
		return nil, &errors.NotAuthorizedError{}
	}
	return details, nil
}

func (m *manager) GetProject(ctx context.Context, projectId edgedb.UUID, target interface{}) error {
	var q string
	switch target.(type) {
	case *LastUpdatedAtField:
		q = `select Project { last_updated_at } filter .id = <uuid>$0`
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
filter .id = <uuid>$0
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
filter .id = <uuid>$0
`
	default:
		return errors.New("missing query for target")
	}
	if err := m.c.QuerySingle(ctx, q, target, projectId); err != nil {
		return rewriteEdgedbError(err)
	}
	return nil
}

func (m *manager) GetProjectAccessForReadAndWriteToken(ctx context.Context, userId edgedb.UUID, token AccessToken) (*TokenAccessResult, error) {
	if err := token.ValidateReadAndWrite(); err != nil {
		return nil, err
	}
	return m.getProjectByToken(ctx, userId, token)
}

func (m *manager) GetProjectAccessForReadOnlyToken(ctx context.Context, userId edgedb.UUID, token AccessToken) (*TokenAccessResult, error) {
	if err := token.ValidateReadOnly(); err != nil {
		return nil, err
	}
	return m.getProjectByToken(ctx, userId, token)
}

func (m *manager) GetEntries(ctx context.Context, projectId, userId edgedb.UUID) (*ForProjectEntries, error) {
	p := &ForProjectEntries{}
	err := m.c.QuerySingle(ctx, `
with
	u := (select User filter .id = <uuid>$1)
select Project {
	docs: { resolved_path },
	files: { resolved_path },
}
filter .id = <uuid>$0 and u in .min_access_ro
`, p, projectId, userId)
	if err != nil {
		return nil, rewriteEdgedbError(err)
	}
	return p, nil
}

func (m *manager) GetProjectMembers(ctx context.Context, projectId edgedb.UUID) ([]user.AsProjectMember, error) {
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
filter .id = <uuid>$0
`, &p, projectId)
	if err != nil {
		return nil, rewriteEdgedbError(err)
	}
	return p.GetProjectMembers(), nil
}

func (m *manager) GrantMemberAccess(ctx context.Context, projectId edgedb.UUID, epoch int64, userId edgedb.UUID, level sharedTypes.PrivilegeLevel) error {
	var q string
	switch level {
	case sharedTypes.PrivilegeLevelReadAndWrite:
		q = `
with
	u := (select User filter .id = <uuid>$0)
update Project
filter
	.id = <uuid>$1
and .epoch = <int64>$2
set {
	epoch := Project.epoch + 1,
	access_rw := distinct (Project.access_rw union {u}),
	access_ro -= u,
}
`
	case sharedTypes.PrivilegeLevelReadOnly:
		q = `
with
	u := (select User filter .id = <uuid>$0)
update Project
filter
	.id = <uuid>$1
and .epoch = <int64>$2
set {
	epoch := Project.epoch + 1,
	access_ro := distinct (Project.access_rw union {u}),
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

func (m *manager) getProjectByToken(ctx context.Context, userId edgedb.UUID, token AccessToken) (*TokenAccessResult, error) {
	p := &forTokenAccessCheck{}
	var tokenPrefixRW, tokenRO AccessToken
	if len(token) == lenReadOnly {
		tokenRO = token
	} else {
		tokenPrefixRW = token[:lenReadAndWritePrefix]
	}
	err := m.c.QuerySingle(ctx, `
with
	u := (select User filter .id = <uuid>$0)
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
	and (
		.tokens.token_prefix_rw = <str>$1 or .tokens.token_ro = <str>$2
	)
limit 1
`, p, userId, tokenPrefixRW, tokenRO)
	if err != nil {
		return nil, rewriteEdgedbError(err)
	}
	freshAccess, err := p.GetPrivilegeLevelAnonymous(token)
	if err != nil {
		return nil, err
	}
	r := &TokenAccessResult{
		ProjectId: p.Id,
		Epoch:     p.Epoch,
		Fresh:     freshAccess,
	}
	if userId == (edgedb.UUID{}) {
		return r, nil
	}
	r.Existing, _ = p.GetPrivilegeLevelAuthenticated(userId)
	return r, nil
}

func (m *manager) GrantReadAndWriteTokenAccess(ctx context.Context, projectId edgedb.UUID, epoch int64, userId edgedb.UUID) error {
	err := rewriteEdgedbError(m.c.QuerySingle(ctx, `
update Project
filter .id = <uuid>$0 and .epoch = <int64>$1
set {
	access_token_rw += (select User filter .id = <uuid>$2),
	epoch := Project.epoch + 1,
}
`, &IdField{}, projectId, epoch, userId))
	if err != nil && errors.IsNotFoundError(err) {
		return ErrEpochIsNotStable
	}
	return err
}

func (m *manager) GrantReadOnlyTokenAccess(ctx context.Context, projectId edgedb.UUID, epoch int64, userId edgedb.UUID) error {
	err := rewriteEdgedbError(m.c.QuerySingle(ctx, `
update Project
filter .id = <uuid>$0 and .epoch = <int64>$1
set {
	access_token_ro += (select User filter .id = <uuid>$2),
	epoch := Project.epoch + 1,
}
`, &IdField{}, projectId, epoch, userId))
	if err != nil && errors.IsNotFoundError(err) {
		return ErrEpochIsNotStable
	}
	return err
}

func (m *manager) RemoveMember(ctx context.Context, projectId edgedb.UUID, epoch int64, userId edgedb.UUID) error {
	var r []bool
	err := m.c.Query(ctx, `
with
	u := (select User filter .id = <uuid>$0),
	p := (
		update Project
		filter
			.id = <uuid>$1
		and .epoch = <int64>$2
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
	tags := (
		update Tag
		filter .user = u
		set {
			projects -= p
		}
	)
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

func (m *manager) SoftDelete(ctx context.Context, projectId, userId edgedb.UUID, ipAddress string) error {
	r := softDeleteResult{}
	err := m.c.QuerySingle(ctx, `
with
	u := (select User filter .id = <uuid>$0),
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

type hardDeleteResult struct {
	ProjectExists      bool `edgedb:"project_exists"`
	ProjectSoftDeleted bool `edgedb:"project_soft_deleted"`
	Deleted            bool `edgedb:"deleted"`
}

func (m *manager) HardDelete(ctx context.Context, projectId edgedb.UUID) error {
	r := hardDeleteResult{}
	err := m.c.QuerySingle(ctx, `
with
	p := (select Project filter .id = <uuid>$0),
	pSoftDeleted := (select p filter exists .deleted_at),
	pDeleted := (delete pSoftDeleted),
select {
	project_exists := exists p,
	project_soft_deleted := exists pSoftDeleted,
	deleted := exists pDeleted,
}
`, &r, projectId)
	if err != nil {
		return rewriteEdgedbError(err)
	}
	switch {
	case !r.ProjectExists:
		return &errors.UnprocessableEntityError{Msg: "project missing"}
	case !r.ProjectSoftDeleted:
		return &errors.UnprocessableEntityError{Msg: "project not soft deleted"}
	}
	return nil
}

func (m *manager) ProcessSoftDeleted(ctx context.Context, cutOff time.Time, fn func(projectId edgedb.UUID) bool) error {
	ids := make([]edgedb.UUID, 0, 100)
	for {
		ids = ids[:0]
		err := m.c.Query(ctx, `
select (
	select Project
	filter .deleted_at <= <datetime>$0
	order by .deleted_at
	limit 100
).id
`, &ids, cutOff)
		if err != nil {
			return rewriteEdgedbError(err)
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

func (m *manager) GetDeletedProjectsName(ctx context.Context, projectId, userId edgedb.UUID) (Name, error) {
	var name Name
	err := m.c.QuerySingle(ctx, `
with
	u := (select User filter .id = <uuid>$1),
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

func (m *manager) Restore(ctx context.Context, projectId, userId edgedb.UUID, name Name) error {
	r := restoreResult{}
	err := m.c.QuerySingle(ctx, `
with
	u := (select User filter .id = <uuid>$0),
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

func (m *manager) CreateDoc(ctx context.Context, projectId, userId, folderId edgedb.UUID, d *Doc) (sharedTypes.Version, error) {
	result := createTreeElementResult{}
	err := m.c.QuerySingle(ctx, `
with
	p := (select Project filter .id = <uuid>$0),
	u := (select User filter .id = <uuid>$1),
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
		d.Id, _ = result.ElementId.Get()
		v, _ := result.Version.Get()
		return sharedTypes.Version(v), nil
	}
}

func (m *manager) CreateFile(ctx context.Context, projectId, userId, folderId edgedb.UUID, f *FileRef) (sharedTypes.Version, error) {
	result := createTreeElementResult{}
	err := m.c.QuerySingle(ctx, `
with
	p := (select Project filter .id = <uuid>$0),
	u := (select User filter .id = <uuid>$1),
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
		f.Id, _ = result.ElementId.Get()
		v, _ := result.Version.Get()
		return sharedTypes.Version(v), nil
	}
}
