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

package project

import (
	"context"
	"strings"
	"time"

	"github.com/edgedb/edgedb-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/mongoTx"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	spellingTypes "github.com/das7pad/overleaf-go/services/spelling/pkg/types"
)

type Manager interface {
	CreateProject(ctx context.Context, creation *ForCreation) error
	Delete(ctx context.Context, p *ForDeletion) error
	Restore(ctx context.Context, p *ForDeletion) error
	GetInactiveProjects(ctx context.Context, age time.Duration) (<-chan edgedb.UUID, error)
	AddTreeElement(ctx context.Context, projectId edgedb.UUID, version sharedTypes.Version, mongoPath MongoPath, element TreeElement) error
	DeleteTreeElement(ctx context.Context, projectId edgedb.UUID, version sharedTypes.Version, mongoPath MongoPath, element TreeElement) error
	DeleteTreeElementAndRootDoc(ctx context.Context, projectId edgedb.UUID, version sharedTypes.Version, mongoPath MongoPath, element TreeElement) error
	MoveTreeElement(ctx context.Context, projectId edgedb.UUID, version sharedTypes.Version, from, to MongoPath, element TreeElement) error
	RenameTreeElement(ctx context.Context, projectId edgedb.UUID, version sharedTypes.Version, mongoPath MongoPath, name sharedTypes.Filename) error
	GetAuthorizationDetails(ctx context.Context, projectId, userId edgedb.UUID, token AccessToken) (*AuthorizationDetails, error)
	BumpEpoch(ctx context.Context, projectId edgedb.UUID) error
	GetEpoch(ctx context.Context, projectId edgedb.UUID) (int64, error)
	GetDocMeta(ctx context.Context, projectId, docId edgedb.UUID) (*Doc, sharedTypes.PathName, error)
	GetJoinProjectDetails(ctx context.Context, projectId, userId edgedb.UUID) (*JoinProjectViewPrivate, error)
	GetLoadEditorDetails(ctx context.Context, projectId, userId edgedb.UUID) (*LoadEditorViewPrivate, error)
	GetProjectRootFolder(ctx context.Context, projectId edgedb.UUID) (*Folder, sharedTypes.Version, error)
	GetProject(ctx context.Context, projectId edgedb.UUID, target interface{}) error
	GetProjectAccessForReadAndWriteToken(ctx context.Context, userId edgedb.UUID, token AccessToken) (*TokenAccessResult, error)
	GetProjectAccessForReadOnlyToken(ctx context.Context, userId edgedb.UUID, token AccessToken) (*TokenAccessResult, error)
	GetTreeAndAuth(ctx context.Context, projectId, userId edgedb.UUID) (*WithTreeAndAuth, error)
	GrantMemberAccess(ctx context.Context, projectId edgedb.UUID, epoch int64, userId edgedb.UUID, level sharedTypes.PrivilegeLevel) error
	GrantReadAndWriteTokenAccess(ctx context.Context, projectId edgedb.UUID, epoch int64, userId edgedb.UUID) error
	GrantReadOnlyTokenAccess(ctx context.Context, projectId edgedb.UUID, epoch int64, userId edgedb.UUID) error
	PopulateTokens(ctx context.Context, projectId edgedb.UUID) (*Tokens, error)
	ListProjects(ctx context.Context, userId edgedb.UUID) ([]*ListViewPrivate, error)
	GetProjectNames(ctx context.Context, userId edgedb.UUID) (Names, error)
	MarkAsActive(ctx context.Context, projectId edgedb.UUID) error
	MarkAsInActive(ctx context.Context, projectId edgedb.UUID) error
	MarkAsOpened(ctx context.Context, projectId edgedb.UUID) error
	UpdateLastUpdated(ctx context.Context, projectId edgedb.UUID, at time.Time, by edgedb.UUID) error
	SetCompiler(ctx context.Context, projectId edgedb.UUID, compiler sharedTypes.Compiler) error
	SetImageName(ctx context.Context, projectId edgedb.UUID, imageName sharedTypes.ImageName) error
	SetSpellCheckLanguage(ctx context.Context, projectId edgedb.UUID, spellCheckLanguage spellingTypes.SpellCheckLanguage) error
	SetRootDocId(ctx context.Context, projectId edgedb.UUID, version sharedTypes.Version, rootDocId edgedb.UUID) error
	SetPublicAccessLevel(ctx context.Context, projectId edgedb.UUID, epoch int64, level PublicAccessLevel) error
	SetTrackChangesState(ctx context.Context, projectId edgedb.UUID, s TrackChangesState) error
	ArchiveForUser(ctx context.Context, projectId, userId edgedb.UUID) error
	UnArchiveForUser(ctx context.Context, projectId, userId edgedb.UUID) error
	TrashForUser(ctx context.Context, projectId, userId edgedb.UUID) error
	UnTrashForUser(ctx context.Context, projectId, userId edgedb.UUID) error
	Rename(ctx context.Context, projectId, userId edgedb.UUID, name Name) error
	RemoveMember(ctx context.Context, projectId edgedb.UUID, epoch int64, userId edgedb.UUID) error
	TransferOwnership(ctx context.Context, p *ForProjectOwnershipTransfer, newOwnerId edgedb.UUID) error
}

func New(c *edgedb.Client, db *mongo.Database) Manager {
	cSlow := db.Collection("projects", options.Collection().
		SetReadPreference(readpref.SecondaryPreferred()),
	)
	return &manager{
		c:     c,
		cP:    db.Collection("projects"),
		cSlow: cSlow,
	}
}

func rewriteEdgedbError(err error) error {
	if e, ok := err.(edgedb.Error); ok && e.Category(edgedb.NoDataError) {
		return &errors.NotFoundError{}
	}
	return err
}

func rewriteMongoError(err error) error {
	if err == mongo.ErrNoDocuments {
		return &errors.NotFoundError{}
	}
	return err
}

func removeArrayIndex(path MongoPath) MongoPath {
	return path[0:strings.LastIndexByte(string(path), '.')]
}

func matchUsersProjects(userId edgedb.UUID) bson.M {
	//goland:noinspection SpellCheckingInspection
	return bson.M{
		"$or": bson.A{
			OwnerRefField{OwnerRef: userId},
			bson.M{
				"tokenAccessReadAndWrite_refs": userId,
				"publicAccesLevel":             TokenBasedAccess,
			},
			bson.M{
				"tokenAccessReadOnly_refs": userId,
				"publicAccesLevel":         TokenBasedAccess,
			},
			bson.M{
				"collaberator_refs": userId,
			},
			bson.M{
				"readOnly_refs": userId,
			},
		},
	}
}

const (
	inactiveProjectBufferSize = 10
	prefetchN                 = 200
)

type manager struct {
	c     *edgedb.Client
	cP    *mongo.Collection
	cSlow *mongo.Collection
}

func (m *manager) GetInactiveProjects(ctx context.Context, age time.Duration) (<-chan edgedb.UUID, error) {
	cutOff := time.Now().UTC().Add(-age)

	q := bson.M{
		"lastOpened": bson.M{
			// Take care of never opened projects, with a negative match.
			"$not": bson.M{
				"$gt": cutOff,
			},
		},
		"_id": bson.M{
			// Look at projects created before the cutOff only.
			"$lt": primitive.NewObjectIDFromTimestamp(cutOff),
		},
		"active": true,
	}
	p := &IdField{}
	projection := getProjection(p)

	r, errFind := m.cSlow.Find(
		ctx, q, options.Find().
			SetBatchSize(inactiveProjectBufferSize).
			SetProjection(projection),
	)
	if errFind != nil {
		return nil, rewriteMongoError(errFind)
	}
	queue := make(chan edgedb.UUID, inactiveProjectBufferSize)

	// Peek once into the batch, then ignore any errors during background
	//  streaming.
	if !r.Next(ctx) {
		close(queue)
		if err := r.Err(); err != nil {
			return nil, err
		}
		return queue, nil
	}
	if err := r.Decode(p); err != nil {
		close(queue)
		return nil, err
	}

	go func() {
		defer close(queue)
		queue <- p.Id
		for r.Next(ctx) {
			if err := r.Decode(p); err != nil {
				return
			}
			queue <- p.Id
		}
	}()
	return queue, nil
}

type docForInsertion struct {
	Name     sharedTypes.Filename
	Size     int64
	Snapshot sharedTypes.Snapshot
}

type fileForInsertion struct {
	Name sharedTypes.Filename
	Size int64
	Hash sharedTypes.Hash
}

type folderForInsertion struct {
	Id    edgedb.UUID
	Docs  []docForInsertion
	Files []fileForInsertion
}

func (m *manager) CreateProject(ctx context.Context, p *ForCreation) error {
	r, errRootFolder := p.GetRootFolder()
	if errRootFolder != nil {
		return errRootFolder
	}
	err := m.c.Tx(ctx, func(ctx context.Context, tx *edgedb.Tx) error {
		{
			// Insert the Project and RootFolder
			ids := make([]IdField, 2)
			// TODO: fill all the project fields
			err := tx.QuerySingle(
				ctx,
				`
with
	p := (insert Project {}),
	rf := (insert RootFolder { project := p })
select {p, rf}
`,
				&ids,
				p,
			)
			if err != nil {
				return rewriteEdgedbError(err)
			}
			p.Id = ids[0].Id
			p.RootFolder[0].Id = ids[1].Id
		}

		nFoldersWithContent := 0
		nFoldersWithChildren := 1
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

		{
			// Build tree of Folders
			queue := make(chan *Folder, nFoldersWithChildren)
			queue <- r

			eg, pCtx := errgroup.WithContext(ctx)
			for i := 0; i < 5; i++ {
				eg.Go(func() error {
					for folder := range queue {
						names := make([]string, len(folder.Folders))
						for j, f := range folder.Folders {
							names[j] = string(f.Name)
						}
						ids := make([]IdField, len(folder.Folders))
						err := tx.QuerySingle(
							pCtx,
							`
with
	project := (select Project filter .id = <uuid>$0),
	parent := (select FolderLike filter .id = <uuid>$1)
for name in array_unpack(<array<str>>$2) union (
	insert Folder { project := project, parent := parent, name := <str>name }
)`,
							ids,
							p.Id, folder.Id, names,
						)
						if err != nil {
							return rewriteEdgedbError(err)
						}
						for j, f := range folder.Folders {
							f.Id = ids[j].Id
							if len(f.Folders) > 0 {
								queue <- f
							}
						}
					}
					return nil
				})
			}

			err := eg.Wait()
			close(queue)
			for range queue {
				// flush the queue
			}
			if err != nil {
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
					for i, d := range f.Docs {
						fi.Docs[i].Name = d.Name
						fi.Docs[i].Size = d.Size
						fi.Docs[i].Snapshot = d.Snapshot
					}
					for i, file := range f.FileRefs {
						fi.Files[i].Name = file.Name
						fi.Files[i].Size = file.Size
						fi.Files[i].Hash = file.Hash
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
			// Insert Docs and Files
			err := tx.Query(ctx, `
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
			insert File {
				project := project,
				parent := f,
				name := <str>file['name'],
				size := <int64>file['size'],
				hash := <str>file['hash'],
	  		}
		)
	)
)`,
				&ids,
				p.Id, folders,
			)
			if err != nil {
				return rewriteEdgedbError(err)
			}
		}

		{
			// Back-fill ids
			i := 0
			err := r.WalkFolders(func(f *Folder, _ sharedTypes.DirName) error {
				for _, doc := range f.Docs {
					doc.Id = ids[i].Id
					i++
				}
				for _, fileRef := range f.FileRefs {
					fileRef.Id = ids[i].Id
					i++
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}

func (m *manager) PopulateTokens(ctx context.Context, projectId edgedb.UUID) (*Tokens, error) {
	allErrors := &errors.MergedError{}
	for i := 0; i < 10; i++ {
		tokens, err := generateTokens()
		if err != nil {
			allErrors.Add(err)
			continue
		}
		q := bson.M{
			"_id": projectId,
			"tokens.readAndWritePrefix": bson.M{
				"$exists": false,
			},
		}
		u := bson.M{
			"$set": TokensField{Tokens: *tokens},
		}
		r, err := m.cP.UpdateOne(ctx, q, u)
		if err != nil {
			if mongo.IsDuplicateKeyError(err) {
				allErrors.Add(err)
				continue
			}
			return nil, rewriteMongoError(err)
		}
		if r.MatchedCount == 1 {
			return tokens, nil
		}
		return nil, nil
	}
	return nil, errors.Tag(allErrors, "bad random source")
}

func (m *manager) SetCompiler(ctx context.Context, projectId edgedb.UUID, compiler sharedTypes.Compiler) error {
	return m.set(ctx, projectId, CompilerField{
		Compiler: compiler,
	})
}

func (m *manager) SetImageName(ctx context.Context, projectId edgedb.UUID, imageName sharedTypes.ImageName) error {
	return m.set(ctx, projectId, ImageNameField{
		ImageName: imageName,
	})
}

func (m *manager) SetSpellCheckLanguage(ctx context.Context, projectId edgedb.UUID, spellCheckLanguage spellingTypes.SpellCheckLanguage) error {
	return m.set(ctx, projectId, SpellCheckLanguageField{
		SpellCheckLanguage: spellCheckLanguage,
	})
}

func (m *manager) SetRootDocId(ctx context.Context, projectId edgedb.UUID, version sharedTypes.Version, rootDocId edgedb.UUID) error {
	return m.setWithVersionGuard(ctx, projectId, version, bson.M{
		"$set": RootDocIdField{
			RootDocId: rootDocId,
		},
	})
}

func (m *manager) SetPublicAccessLevel(ctx context.Context, projectId edgedb.UUID, epoch int64, publicAccessLevel PublicAccessLevel) error {
	return m.setWithEpochGuard(ctx, projectId, epoch, bson.M{
		"$set": PublicAccessLevelField{
			PublicAccessLevel: publicAccessLevel,
		},
	})
}

func (m *manager) SetTrackChangesState(ctx context.Context, projectId edgedb.UUID, s TrackChangesState) error {
	return m.set(ctx, projectId, &TrackChangesStateField{
		TrackChangesState: s,
	})
}

func (m *manager) TransferOwnership(ctx context.Context, p *ForProjectOwnershipTransfer, newOwnerId edgedb.UUID) error {
	previousOwnerId := p.OwnerRef

	// We need to add the previous owner and remove the new  owner from the
	//  list of collaborators.
	// Mongo does not allow multiple actions on a field in a single update
	//  operation. We need to push and pull. Rewrite the array instead.
	collaboratorRefs := make(Refs, 0, len(p.CollaboratorRefs))
	for _, id := range p.CollaboratorRefs {
		if id != newOwnerId {
			collaboratorRefs = append(collaboratorRefs, id)
		}
	}
	collaboratorRefs = append(collaboratorRefs, previousOwnerId)

	//goland:noinspection SpellCheckingInspection
	u := bson.M{
		// Use the epoch as guard for concurrent write operations, this
		//  includes revoking membership, which is required for the new owner.
		"$inc": EpochField{Epoch: 1},

		"$pull": bson.M{
			"readOnly_refs":                newOwnerId,
			"tokenAccessReadAndWrite_refs": newOwnerId,
			"tokenAccessReadOnly_refs":     newOwnerId,
		},

		"$push": bson.M{
			"auditLog": bson.M{
				"$each": bson.A{
					AuditLogEntry{
						InitiatorId: previousOwnerId,
						Operation:   "transfer-ownership",
						Timestamp:   time.Now().UTC(),
						Info: transferOwnershipAuditLogInfo{
							PreviousOwnerId: previousOwnerId,
							NewOwnerId:      newOwnerId,
						},
					},
				},
				"$slice": -MaxAuditLogEntries,
			},
		},

		"$set": bson.M{
			"owner_ref":         newOwnerId,
			"collaberator_refs": collaboratorRefs,
		},
	}

	return m.setWithEpochGuard(ctx, p.Id, p.Epoch, u)
}

func (m *manager) Rename(ctx context.Context, projectId, userId edgedb.UUID, name Name) error {
	err := m.checkAccessAndUpdate(
		ctx, projectId, userId, sharedTypes.PrivilegeLevelOwner, &bson.M{
			"$set": NameField{
				Name: name,
			},
		},
	)
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}

var ErrVersionChanged = &errors.InvalidStateError{Msg: "project version changed"}

func (m *manager) AddTreeElement(ctx context.Context, projectId edgedb.UUID, version sharedTypes.Version, mongoPath MongoPath, element TreeElement) error {
	return m.setWithVersionGuard(ctx, projectId, version, bson.M{
		"$push": bson.M{
			string(mongoPath + "." + element.FieldNameInFolder()): element,
		},
		"$inc": VersionField{Version: 1},
	})
}

func (m *manager) DeleteTreeElement(ctx context.Context, projectId edgedb.UUID, version sharedTypes.Version, mongoPath MongoPath, element TreeElement) error {
	return m.deleteTreeElementAndMaybeRootDoc(ctx, projectId, version, mongoPath, element, false)
}

func (m *manager) DeleteTreeElementAndRootDoc(ctx context.Context, projectId edgedb.UUID, version sharedTypes.Version, mongoPath MongoPath, element TreeElement) error {
	return m.deleteTreeElementAndMaybeRootDoc(ctx, projectId, version, mongoPath, element, true)
}

func (m *manager) deleteTreeElementAndMaybeRootDoc(ctx context.Context, projectId edgedb.UUID, version sharedTypes.Version, mongoPath MongoPath, element TreeElement, unsetRootDoc bool) error {
	u := bson.M{
		"$pull": bson.M{
			string(removeArrayIndex(mongoPath)): CommonTreeFields{
				Id:   element.GetId(),
				Name: element.GetName(),
			},
		},
		"$inc": VersionField{Version: 1},
	}
	if unsetRootDoc {
		u["$unset"] = bson.M{
			"rootDoc_id": true,
		}
	}
	return m.setWithVersionGuard(ctx, projectId, version, u)
}

func (m *manager) MoveTreeElement(ctx context.Context, projectId edgedb.UUID, version sharedTypes.Version, from, to MongoPath, element TreeElement) error {
	// NOTE: Mongo allows one operation per field only.
	//       We need to push/pull into/from the same rootFolder.
	//       Use a transaction for the move operation.
	// NOTE: Array indexes can change after pulling. Push first.
	u1 := &bson.M{
		"$push": bson.M{
			string(to + "." + element.FieldNameInFolder()): element,
		},
	}
	u2 := &bson.M{
		"$pull": bson.M{
			string(removeArrayIndex(from)): CommonTreeFields{
				Id:   element.GetId(),
				Name: element.GetName(),
			},
		},
		"$inc": VersionField{Version: 1},
	}
	err := mongoTx.For(m.cP.Database(), ctx, func(ctx context.Context) error {
		for _, u := range []interface{}{u1, u2} {
			err := m.setWithVersionGuard(ctx, projectId, version, u)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (m *manager) RenameTreeElement(ctx context.Context, projectId edgedb.UUID, version sharedTypes.Version, mongoPath MongoPath, name sharedTypes.Filename) error {
	return m.setWithVersionGuard(ctx, projectId, version, bson.M{
		"$set": bson.M{
			string(mongoPath) + ".name": name,
		},
		"$inc": VersionField{Version: 1},
	})
}

func (m *manager) ArchiveForUser(ctx context.Context, projectId, userId edgedb.UUID) error {
	return m.checkAccessAndUpdate(
		ctx, projectId, userId, sharedTypes.PrivilegeLevelReadOnly, &bson.M{
			"$addToSet": bson.M{
				"archived": userId,
			},
			"$pull": bson.M{
				"trashed": userId,
			},
		},
	)
}

func (m *manager) UnArchiveForUser(ctx context.Context, projectId, userId edgedb.UUID) error {
	return m.checkAccessAndUpdate(
		ctx, projectId, userId, sharedTypes.PrivilegeLevelReadOnly, &bson.M{
			"$pull": bson.M{
				"archived": userId,
			},
		},
	)
}

func (m *manager) TrashForUser(ctx context.Context, projectId, userId edgedb.UUID) error {
	return m.checkAccessAndUpdate(
		ctx, projectId, userId, sharedTypes.PrivilegeLevelReadOnly, &bson.M{
			"$addToSet": bson.M{
				"trashed": userId,
			},
			"$pull": bson.M{
				"archived": userId,
			},
		},
	)
}

func (m *manager) UnTrashForUser(ctx context.Context, projectId, userId edgedb.UUID) error {
	return m.checkAccessAndUpdate(
		ctx, projectId, userId, sharedTypes.PrivilegeLevelReadOnly, &bson.M{
			"$pull": bson.M{
				"trashed": userId,
			},
		},
	)
}

var ErrEpochIsNotStable = errors.New("epoch is not stable")

func (m *manager) setWithEpochGuard(ctx context.Context, projectId edgedb.UUID, epoch int64, u interface{}) error {
	q := withIdAndEpoch{}
	q.Id = projectId
	q.Epoch = epoch
	r, err := m.cP.UpdateOne(ctx, q, u)
	if err != nil {
		return rewriteMongoError(err)
	}
	if r.MatchedCount != 1 {
		return ErrEpochIsNotStable
	}
	return nil
}

func (m *manager) setWithVersionGuard(ctx context.Context, projectId edgedb.UUID, version sharedTypes.Version, u interface{}) error {
	q := &withIdAndVersion{}
	q.Id = projectId
	q.Version = version
	r, err := m.cP.UpdateOne(ctx, q, u)
	if err != nil {
		return rewriteMongoError(err)
	}
	if r.MatchedCount != 1 {
		return ErrVersionChanged
	}
	return nil
}

func (m *manager) checkAccessAndUpdate(ctx context.Context, projectId, userId edgedb.UUID, minLevel sharedTypes.PrivilegeLevel, u interface{}) error {
	for i := 0; i < 10; i++ {
		p := &ForAuthorizationDetails{}
		qId := &IdField{Id: projectId}
		err := m.fetchWithMinimalAuthorizationDetails(ctx, qId, userId, p)
		if err != nil {
			return err
		}
		if err = p.CheckPrivilegeLevelIsAtLest(userId, minLevel); err != nil {
			return err
		}
		err = m.setWithEpochGuard(ctx, projectId, p.Epoch, u)
		if err != nil {
			if err == ErrEpochIsNotStable {
				continue
			}
			return err
		}
		return nil
	}
	return ErrEpochIsNotStable
}

func (m *manager) GetProjectNames(ctx context.Context, userId edgedb.UUID) (Names, error) {
	q := matchUsersProjects(userId)
	var projects []NameField
	r, err := m.cP.Find(
		ctx,
		q,
		options.Find().
			SetProjection(getProjection(projects)).
			SetBatchSize(prefetchN),
	)
	if err != nil {
		return nil, rewriteMongoError(err)
	}
	if err = r.All(ctx, &projects); err != nil {
		return nil, rewriteMongoError(err)
	}
	names := make(Names, len(projects))
	for i, project := range projects {
		names[i] = project.Name
	}
	return names, nil
}

func (m *manager) ListProjects(ctx context.Context, userId edgedb.UUID) ([]*ListViewPrivate, error) {
	var projects []*ListViewPrivate
	projection := getProjection(projects).CloneForWriting()

	limitToUser := &bson.M{
		"$elemMatch": bson.M{
			"$eq": userId,
		},
	}
	// These fields are used for an authorization check only, we do not
	//  need to fetch all of them.
	for s := range withMembersProjection {
		projection[s] = limitToUser
	}
	projection["_id"] = true
	projection["archived"] = limitToUser
	projection["trashed"] = limitToUser

	q := matchUsersProjects(userId)

	r, err := m.cP.Find(
		ctx,
		q,
		options.Find().SetProjection(projection).SetBatchSize(prefetchN),
	)
	if err != nil {
		return projects, rewriteMongoError(err)
	}
	if err = r.All(ctx, &projects); err != nil {
		return projects, rewriteMongoError(err)
	}
	return projects, nil
}

func (m *manager) GetAuthorizationDetails(ctx context.Context, projectId, userId edgedb.UUID, token AccessToken) (*AuthorizationDetails, error) {
	p := &ForAuthorizationDetails{}
	q := &IdField{Id: projectId}
	err := m.fetchWithMinimalAuthorizationDetails(ctx, q, userId, p)
	if err != nil {
		return nil, errors.Tag(err, "cannot get project from mongo")
	}
	return p.GetPrivilegeLevel(userId, token)
}

func (m *manager) BumpEpoch(ctx context.Context, projectId edgedb.UUID) error {
	_, err := m.cP.UpdateOne(ctx, &IdField{Id: projectId}, &bson.M{
		"$inc": &EpochField{Epoch: 1},
	})
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}

func (m *manager) GetEpoch(ctx context.Context, projectId edgedb.UUID) (int64, error) {
	p := &EpochField{}
	err := m.GetProject(ctx, projectId, p)
	return p.Epoch, err
}

func (m *manager) set(ctx context.Context, projectId edgedb.UUID, update interface{}) error {
	_, err := m.cP.UpdateOne(
		ctx,
		IdField{
			Id: projectId,
		},
		bson.M{
			"$set": update,
		},
	)
	return err
}

func (m *manager) UpdateLastUpdated(ctx context.Context, projectId edgedb.UUID, at time.Time, by edgedb.UUID) error {
	v := WithLastUpdatedDetails{}
	v.LastUpdatedAt = at
	v.LastUpdatedBy = by
	_, err := m.cP.UpdateOne(
		ctx,
		bson.M{
			"_id": projectId,
			"lastUpdated": bson.M{
				"$gt": at,
			},
		},
		bson.M{
			"$set": v,
		},
	)
	return err
}

func (m *manager) GetDocMeta(ctx context.Context, projectId, docId edgedb.UUID) (*Doc, sharedTypes.PathName, error) {
	f, _, err := m.GetProjectRootFolder(ctx, projectId)
	if err != nil {
		return nil, "", errors.Tag(err, "cannot get tree")
	}
	var doc *Doc
	var p sharedTypes.PathName
	err = f.WalkDocs(func(element TreeElement, path sharedTypes.PathName) error {
		if element.GetId() == docId {
			doc = element.(*Doc)
			p = path
			return AbortWalk
		}
		return nil
	})
	if err != nil {
		return nil, "", errors.Tag(err, "cannot walk project tree")
	}
	if doc == nil {
		return nil, "", &errors.NotFoundError{}
	}
	return doc, p, nil
}

func (m *manager) GetProjectRootFolder(ctx context.Context, projectId edgedb.UUID) (*Folder, sharedTypes.Version, error) {
	var project WithTree
	err := m.cP.FindOne(
		ctx,
		bson.M{
			"_id": projectId,
		},
		options.FindOne().SetProjection(getProjection(project)),
	).Decode(&project)
	if err != nil {
		return nil, 0, rewriteMongoError(err)
	}
	t, err := project.GetRootFolder()
	if err != nil {
		return nil, 0, err
	}
	return t, project.Version, nil
}

func (m *manager) GetJoinProjectDetails(ctx context.Context, projectId, userId edgedb.UUID) (*JoinProjectViewPrivate, error) {
	project := &JoinProjectViewPrivate{}
	err := m.fetchWithMinimalAuthorizationDetails(
		ctx, &IdField{Id: projectId}, userId, project,
	)
	if err != nil {
		return nil, err
	}
	return project, nil
}

func (m *manager) GetLoadEditorDetails(ctx context.Context, projectId, userId edgedb.UUID) (*LoadEditorViewPrivate, error) {
	project := &LoadEditorViewPrivate{}
	err := m.fetchWithMinimalAuthorizationDetails(
		ctx, &IdField{Id: projectId}, userId, project,
	)
	if err != nil {
		return nil, err
	}
	return project, nil
}

func (m *manager) fetchWithMinimalAuthorizationDetails(ctx context.Context, q interface{}, userId edgedb.UUID, target interface{}) error {
	projection := getProjection(target).CloneForWriting()
	if userId == (edgedb.UUID{}) {
		for s := range withMembersProjection {
			delete(projection, s)
		}
	} else {
		limitToUser := &bson.M{
			"$elemMatch": bson.M{
				"$eq": userId,
			},
		}
		getId := projection["_id"]
		// These fields are used for an authorization check only, we do not
		//  need to fetch all of them.
		for s := range withTokenMembersProjection {
			projection[s] = limitToUser
		}
		projection["_id"] = getId
	}

	err := m.cP.FindOne(
		ctx,
		q,
		options.FindOne().SetProjection(projection),
	).Decode(target)
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}

func (m *manager) GetProject(ctx context.Context, projectId edgedb.UUID, target interface{}) error {
	err := m.cP.FindOne(
		ctx,
		IdField{Id: projectId},
		options.FindOne().SetProjection(getProjection(target)),
	).Decode(target)
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}

func (m *manager) GetProjectAccessForReadAndWriteToken(ctx context.Context, userId edgedb.UUID, token AccessToken) (*TokenAccessResult, error) {
	if err := token.ValidateReadAndWrite(); err != nil {
		return nil, err
	}
	q := &bson.M{
		"$and": bson.A{
			PublicAccessLevelField{PublicAccessLevel: TokenBasedAccess},
			bson.M{"tokens.readAndWritePrefix": token[0:10]},
		},
	}
	return m.getProjectByToken(ctx, q, userId, token)
}

func (m *manager) GetProjectAccessForReadOnlyToken(ctx context.Context, userId edgedb.UUID, token AccessToken) (*TokenAccessResult, error) {
	if err := token.ValidateReadOnly(); err != nil {
		return nil, err
	}
	q := &bson.M{
		"$and": bson.A{
			PublicAccessLevelField{PublicAccessLevel: TokenBasedAccess},
			bson.M{"tokens.readOnly": token},
		},
	}
	return m.getProjectByToken(ctx, q, userId, token)
}

func (m *manager) GetTreeAndAuth(ctx context.Context, projectId, userId edgedb.UUID) (*WithTreeAndAuth, error) {
	p := &WithTreeAndAuth{}
	q := &IdField{Id: projectId}
	err := m.fetchWithMinimalAuthorizationDetails(ctx, q, userId, p)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (m *manager) GrantMemberAccess(ctx context.Context, projectId edgedb.UUID, epoch int64, userId edgedb.UUID, level sharedTypes.PrivilegeLevel) error {
	u := bson.M{
		// Use the epoch as guard for concurrent write operations, this
		//  includes revoking of invitations.
		"$inc": EpochField{Epoch: 1},
	}
	switch level {
	case sharedTypes.PrivilegeLevelReadAndWrite:
		//goland:noinspection SpellCheckingInspection
		u["$addToSet"] = bson.M{
			"collaberator_refs": userId,
		}
		u["$pull"] = bson.M{
			"readOnly_refs": userId,
		}
	case sharedTypes.PrivilegeLevelReadOnly:
		u["$addToSet"] = bson.M{
			"readOnly_refs": userId,
		}
		//goland:noinspection SpellCheckingInspection
		u["$pull"] = bson.M{
			"collaberator_refs": userId,
		}
	default:
		return errors.New("invalid member access level: " + string(level))
	}

	return m.setWithEpochGuard(ctx, projectId, epoch, u)
}

func (m *manager) getProjectByToken(ctx context.Context, q interface{}, userId edgedb.UUID, token AccessToken) (*TokenAccessResult, error) {
	p := &forTokenAccessCheck{}
	err := m.fetchWithMinimalAuthorizationDetails(ctx, q, userId, p)
	if err != nil {
		return nil, rewriteMongoError(err)
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
	u := &bson.M{
		"$inc": EpochField{Epoch: 1},
		"$addToSet": &bson.M{
			"tokenAccessReadAndWrite_refs": userId,
		},
	}
	return m.setWithEpochGuard(ctx, projectId, epoch, u)
}

func (m *manager) GrantReadOnlyTokenAccess(ctx context.Context, projectId edgedb.UUID, epoch int64, userId edgedb.UUID) error {
	u := &bson.M{
		"$inc": EpochField{Epoch: 1},
		"$addToSet": &bson.M{
			"tokenAccessReadOnly_refs": userId,
		},
	}
	return m.setWithEpochGuard(ctx, projectId, epoch, u)
}

func (m *manager) MarkAsActive(ctx context.Context, projectId edgedb.UUID) error {
	return m.set(ctx, projectId, &ActiveField{Active: true})
}

func (m *manager) MarkAsInActive(ctx context.Context, projectId edgedb.UUID) error {
	return m.set(ctx, projectId, &ActiveField{Active: false})
}

func (m *manager) MarkAsOpened(ctx context.Context, projectId edgedb.UUID) error {
	return m.set(ctx, projectId, &LastOpenedField{LastOpened: time.Now()})
}

func (m *manager) RemoveMember(ctx context.Context, projectId edgedb.UUID, epoch int64, userId edgedb.UUID) error {
	u := bson.M{
		"$inc": EpochField{Epoch: 1},
	}

	pull := bson.M{}
	for s := range forMemberRemovalFields {
		pull[s] = userId
	}
	u["$pull"] = pull

	return m.setWithEpochGuard(ctx, projectId, epoch, u)
}

func (m *manager) Delete(ctx context.Context, p *ForDeletion) error {
	q := &withIdAndEpochAndVersion{
		IdField:      p.IdField,
		EpochField:   p.EpochField,
		VersionField: p.VersionField,
	}
	r, err := m.cP.DeleteOne(ctx, q)
	if err != nil {
		return rewriteMongoError(err)
	}
	if r.DeletedCount != 1 {
		return ErrVersionChanged
	}
	return nil
}

func (m *manager) Restore(ctx context.Context, p *ForDeletion) error {
	_, err := m.cP.InsertOne(ctx, p)
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}
