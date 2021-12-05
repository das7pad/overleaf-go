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

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/mongoTx"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	clsiTypes "github.com/das7pad/overleaf-go/services/clsi/pkg/types"
	spellingTypes "github.com/das7pad/overleaf-go/services/spelling/pkg/types"
)

type Manager interface {
	CreateProject(ctx context.Context, creation *ForCreation) error
	Delete(ctx context.Context, p *ForDeletion) error
	Restore(ctx context.Context, p *ForDeletion) error
	AddTreeElement(ctx context.Context, projectId primitive.ObjectID, version sharedTypes.Version, mongoPath MongoPath, element TreeElement) error
	DeleteTreeElement(ctx context.Context, projectId primitive.ObjectID, version sharedTypes.Version, mongoPath MongoPath, element TreeElement) error
	DeleteTreeElementAndRootDoc(ctx context.Context, projectId primitive.ObjectID, version sharedTypes.Version, mongoPath MongoPath, element TreeElement) error
	MoveTreeElement(ctx context.Context, projectId primitive.ObjectID, version sharedTypes.Version, from, to MongoPath, element TreeElement) error
	RenameTreeElement(ctx context.Context, projectId primitive.ObjectID, version sharedTypes.Version, mongoPath MongoPath, name sharedTypes.Filename) error
	GetAuthorizationDetails(ctx context.Context, projectId, userId primitive.ObjectID, token AccessToken) (*AuthorizationDetails, error)
	BumpEpoch(ctx context.Context, projectId primitive.ObjectID) error
	GetEpoch(ctx context.Context, projectId primitive.ObjectID) (int64, error)
	GetDocMeta(ctx context.Context, projectId, docId primitive.ObjectID) (*Doc, sharedTypes.PathName, error)
	GetJoinProjectDetails(ctx context.Context, projectId, userId primitive.ObjectID) (*JoinProjectViewPrivate, error)
	GetLoadEditorDetails(ctx context.Context, projectId, userId primitive.ObjectID) (*LoadEditorViewPrivate, error)
	GetProjectRootFolder(ctx context.Context, projectId primitive.ObjectID) (*Folder, sharedTypes.Version, error)
	GetProject(ctx context.Context, projectId primitive.ObjectID, target interface{}) error
	GetProjectAccessForReadAndWriteToken(ctx context.Context, userId primitive.ObjectID, token AccessToken) (*TokenAccessResult, error)
	GetProjectAccessForReadOnlyToken(ctx context.Context, userId primitive.ObjectID, token AccessToken) (*TokenAccessResult, error)
	GetTreeAndAuth(ctx context.Context, projectId, userId primitive.ObjectID) (*WithTreeAndAuth, error)
	GrantMemberAccess(ctx context.Context, projectId primitive.ObjectID, epoch int64, userId primitive.ObjectID, level sharedTypes.PrivilegeLevel) error
	GrantReadAndWriteTokenAccess(ctx context.Context, projectId primitive.ObjectID, epoch int64, userId primitive.ObjectID) error
	GrantReadOnlyTokenAccess(ctx context.Context, projectId primitive.ObjectID, epoch int64, userId primitive.ObjectID) error
	PopulateTokens(ctx context.Context, projectId primitive.ObjectID) (*Tokens, error)
	ListProjects(ctx context.Context, userId primitive.ObjectID) ([]*ListViewPrivate, error)
	GetProjectNames(ctx context.Context, userId primitive.ObjectID) (Names, error)
	MarkAsActive(ctx context.Context, projectId primitive.ObjectID) error
	MarkAsInActive(ctx context.Context, projectId primitive.ObjectID) error
	MarkAsOpened(ctx context.Context, projectId primitive.ObjectID) error
	UpdateLastUpdated(ctx context.Context, projectId primitive.ObjectID, at time.Time, by primitive.ObjectID) error
	SetCompiler(ctx context.Context, projectId primitive.ObjectID, compiler clsiTypes.Compiler) error
	SetImageName(ctx context.Context, projectId primitive.ObjectID, imageName clsiTypes.ImageName) error
	SetSpellCheckLanguage(ctx context.Context, projectId primitive.ObjectID, spellCheckLanguage spellingTypes.SpellCheckLanguage) error
	SetRootDocId(ctx context.Context, projectId primitive.ObjectID, version sharedTypes.Version, rootDocId primitive.ObjectID) error
	SetPublicAccessLevel(ctx context.Context, projectId primitive.ObjectID, epoch int64, level PublicAccessLevel) error
	ArchiveForUser(ctx context.Context, projectId, userId primitive.ObjectID) error
	UnArchiveForUser(ctx context.Context, projectId, userId primitive.ObjectID) error
	TrashForUser(ctx context.Context, projectId, userId primitive.ObjectID) error
	UnTrashForUser(ctx context.Context, projectId, userId primitive.ObjectID) error
	Rename(ctx context.Context, projectId, userId primitive.ObjectID, name Name) error
	RemoveMember(ctx context.Context, projectId primitive.ObjectID, epoch int64, userId primitive.ObjectID) error
	TransferOwnership(ctx context.Context, p *ForProjectOwnershipTransfer, newOwnerId primitive.ObjectID) error
}

func New(db *mongo.Database) Manager {
	return &manager{
		c: db.Collection("projects"),
	}
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

func matchUsersProjects(userId primitive.ObjectID) bson.M {
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

type manager struct {
	c *mongo.Collection
}

func (m *manager) CreateProject(ctx context.Context, p *ForCreation) error {
	_, err := m.c.InsertOne(ctx, p)
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}

func (m *manager) PopulateTokens(ctx context.Context, projectId primitive.ObjectID) (*Tokens, error) {
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
		r, err := m.c.UpdateOne(ctx, q, u)
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

func (m *manager) SetCompiler(ctx context.Context, projectId primitive.ObjectID, compiler clsiTypes.Compiler) error {
	return m.set(ctx, projectId, CompilerField{
		Compiler: compiler,
	})
}

func (m *manager) SetImageName(ctx context.Context, projectId primitive.ObjectID, imageName clsiTypes.ImageName) error {
	return m.set(ctx, projectId, ImageNameField{
		ImageName: imageName,
	})
}

func (m *manager) SetSpellCheckLanguage(ctx context.Context, projectId primitive.ObjectID, spellCheckLanguage spellingTypes.SpellCheckLanguage) error {
	return m.set(ctx, projectId, SpellCheckLanguageField{
		SpellCheckLanguage: spellCheckLanguage,
	})
}

func (m *manager) SetRootDocId(ctx context.Context, projectId primitive.ObjectID, version sharedTypes.Version, rootDocId primitive.ObjectID) error {
	return m.setWithVersionGuard(ctx, projectId, version, bson.M{
		"$set": RootDocIdField{
			RootDocId: rootDocId,
		},
	})
}

func (m *manager) SetPublicAccessLevel(ctx context.Context, projectId primitive.ObjectID, epoch int64, publicAccessLevel PublicAccessLevel) error {
	return m.setWithEpochGuard(ctx, projectId, epoch, bson.M{
		"$set": PublicAccessLevelField{
			PublicAccessLevel: publicAccessLevel,
		},
	})
}

func (m *manager) TransferOwnership(ctx context.Context, p *ForProjectOwnershipTransfer, newOwnerId primitive.ObjectID) error {
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

func (m *manager) Rename(ctx context.Context, projectId, userId primitive.ObjectID, name Name) error {
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

func (m *manager) AddTreeElement(ctx context.Context, projectId primitive.ObjectID, version sharedTypes.Version, mongoPath MongoPath, element TreeElement) error {
	return m.setWithVersionGuard(ctx, projectId, version, bson.M{
		"$push": bson.M{
			string(mongoPath + "." + element.FieldNameInFolder()): element,
		},
		"$inc": VersionField{Version: 1},
	})
}

func (m *manager) DeleteTreeElement(ctx context.Context, projectId primitive.ObjectID, version sharedTypes.Version, mongoPath MongoPath, element TreeElement) error {
	return m.deleteTreeElementAndMaybeRootDoc(ctx, projectId, version, mongoPath, element, false)
}

func (m *manager) DeleteTreeElementAndRootDoc(ctx context.Context, projectId primitive.ObjectID, version sharedTypes.Version, mongoPath MongoPath, element TreeElement) error {
	return m.deleteTreeElementAndMaybeRootDoc(ctx, projectId, version, mongoPath, element, true)
}

func (m *manager) deleteTreeElementAndMaybeRootDoc(ctx context.Context, projectId primitive.ObjectID, version sharedTypes.Version, mongoPath MongoPath, element TreeElement, unsetRootDoc bool) error {
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

func (m *manager) MoveTreeElement(ctx context.Context, projectId primitive.ObjectID, version sharedTypes.Version, from, to MongoPath, element TreeElement) error {
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
	err := mongoTx.For(m.c.Database(), ctx, func(ctx context.Context) error {
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

func (m *manager) RenameTreeElement(ctx context.Context, projectId primitive.ObjectID, version sharedTypes.Version, mongoPath MongoPath, name sharedTypes.Filename) error {
	return m.setWithVersionGuard(ctx, projectId, version, bson.M{
		"$set": bson.M{
			string(mongoPath) + ".name": name,
		},
		"$inc": VersionField{Version: 1},
	})
}

func (m *manager) ArchiveForUser(ctx context.Context, projectId, userId primitive.ObjectID) error {
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

func (m *manager) UnArchiveForUser(ctx context.Context, projectId, userId primitive.ObjectID) error {
	return m.checkAccessAndUpdate(
		ctx, projectId, userId, sharedTypes.PrivilegeLevelReadOnly, &bson.M{
			"$pull": bson.M{
				"archived": userId,
			},
		},
	)
}

func (m *manager) TrashForUser(ctx context.Context, projectId, userId primitive.ObjectID) error {
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

func (m *manager) UnTrashForUser(ctx context.Context, projectId, userId primitive.ObjectID) error {
	return m.checkAccessAndUpdate(
		ctx, projectId, userId, sharedTypes.PrivilegeLevelReadOnly, &bson.M{
			"$pull": bson.M{
				"trashed": userId,
			},
		},
	)
}

var ErrEpochIsNotStable = errors.New("epoch is not stable")

func (m *manager) setWithEpochGuard(ctx context.Context, projectId primitive.ObjectID, epoch int64, u interface{}) error {
	q := withIdAndEpoch{}
	q.Id = projectId
	q.Epoch = epoch
	r, err := m.c.UpdateOne(ctx, q, u)
	if err != nil {
		return rewriteMongoError(err)
	}
	if r.MatchedCount != 1 {
		return ErrEpochIsNotStable
	}
	return nil
}

func (m *manager) setWithVersionGuard(ctx context.Context, projectId primitive.ObjectID, version sharedTypes.Version, u interface{}) error {
	q := &withIdAndVersion{}
	q.Id = projectId
	q.Version = version
	r, err := m.c.UpdateOne(ctx, q, u)
	if err != nil {
		return rewriteMongoError(err)
	}
	if r.MatchedCount != 1 {
		return ErrVersionChanged
	}
	return nil
}

func (m *manager) checkAccessAndUpdate(ctx context.Context, projectId, userId primitive.ObjectID, minLevel sharedTypes.PrivilegeLevel, u interface{}) error {
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

func (m *manager) GetProjectNames(ctx context.Context, userId primitive.ObjectID) (Names, error) {
	q := matchUsersProjects(userId)
	var projects []NameField
	r, err := m.c.Find(
		ctx, q, options.Find().SetProjection(getProjection(projects)),
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

func (m *manager) ListProjects(ctx context.Context, userId primitive.ObjectID) ([]*ListViewPrivate, error) {
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

	r, err := m.c.Find(ctx, q, options.Find().SetProjection(projection))
	if err != nil {
		return projects, rewriteMongoError(err)
	}
	if err = r.All(ctx, &projects); err != nil {
		return projects, rewriteMongoError(err)
	}
	return projects, nil
}

func (m *manager) GetAuthorizationDetails(ctx context.Context, projectId, userId primitive.ObjectID, token AccessToken) (*AuthorizationDetails, error) {
	p := &ForAuthorizationDetails{}
	q := &IdField{Id: projectId}
	err := m.fetchWithMinimalAuthorizationDetails(ctx, q, userId, p)
	if err != nil {
		return nil, errors.Tag(err, "cannot get project from mongo")
	}
	return p.GetPrivilegeLevel(userId, token)
}

func (m *manager) BumpEpoch(ctx context.Context, projectId primitive.ObjectID) error {
	_, err := m.c.UpdateOne(ctx, &IdField{Id: projectId}, &bson.M{
		"$inc": &EpochField{Epoch: 1},
	})
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}

func (m *manager) GetEpoch(ctx context.Context, projectId primitive.ObjectID) (int64, error) {
	p := &EpochField{}
	err := m.GetProject(ctx, projectId, p)
	return p.Epoch, err
}

func (m *manager) set(ctx context.Context, projectId primitive.ObjectID, update interface{}) error {
	_, err := m.c.UpdateOne(
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

func (m *manager) UpdateLastUpdated(ctx context.Context, projectId primitive.ObjectID, at time.Time, by primitive.ObjectID) error {
	v := WithLastUpdatedDetails{}
	v.LastUpdatedAt = at
	v.LastUpdatedBy = by
	_, err := m.c.UpdateOne(
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

func (m *manager) GetDocMeta(ctx context.Context, projectId, docId primitive.ObjectID) (*Doc, sharedTypes.PathName, error) {
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

func (m *manager) GetProjectRootFolder(ctx context.Context, projectId primitive.ObjectID) (*Folder, sharedTypes.Version, error) {
	var project WithTree
	err := m.c.FindOne(
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

func (m *manager) GetJoinProjectDetails(ctx context.Context, projectId, userId primitive.ObjectID) (*JoinProjectViewPrivate, error) {
	project := &JoinProjectViewPrivate{}
	err := m.fetchWithMinimalAuthorizationDetails(
		ctx, &IdField{Id: projectId}, userId, project,
	)
	if err != nil {
		return nil, err
	}
	return project, nil
}

func (m *manager) GetLoadEditorDetails(ctx context.Context, projectId, userId primitive.ObjectID) (*LoadEditorViewPrivate, error) {
	project := &LoadEditorViewPrivate{}
	err := m.fetchWithMinimalAuthorizationDetails(
		ctx, &IdField{Id: projectId}, userId, project,
	)
	if err != nil {
		return nil, err
	}
	return project, nil
}

func (m *manager) fetchWithMinimalAuthorizationDetails(ctx context.Context, q interface{}, userId primitive.ObjectID, target interface{}) error {
	projection := getProjection(target).CloneForWriting()
	if userId.IsZero() {
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

	err := m.c.FindOne(
		ctx,
		q,
		options.FindOne().SetProjection(projection),
	).Decode(target)
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}

func (m *manager) GetProject(ctx context.Context, projectId primitive.ObjectID, target interface{}) error {
	err := m.c.FindOne(
		ctx,
		IdField{Id: projectId},
		options.FindOne().SetProjection(getProjection(target)),
	).Decode(target)
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}

func (m *manager) GetProjectAccessForReadAndWriteToken(ctx context.Context, userId primitive.ObjectID, token AccessToken) (*TokenAccessResult, error) {
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

func (m *manager) GetProjectAccessForReadOnlyToken(ctx context.Context, userId primitive.ObjectID, token AccessToken) (*TokenAccessResult, error) {
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

func (m *manager) GetTreeAndAuth(ctx context.Context, projectId, userId primitive.ObjectID) (*WithTreeAndAuth, error) {
	p := &WithTreeAndAuth{}
	q := &IdField{Id: projectId}
	err := m.fetchWithMinimalAuthorizationDetails(ctx, q, userId, p)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (m *manager) GrantMemberAccess(ctx context.Context, projectId primitive.ObjectID, epoch int64, userId primitive.ObjectID, level sharedTypes.PrivilegeLevel) error {
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

func (m *manager) getProjectByToken(ctx context.Context, q interface{}, userId primitive.ObjectID, token AccessToken) (*TokenAccessResult, error) {
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
	if userId.IsZero() {
		return r, nil
	}
	r.Existing, _ = p.GetPrivilegeLevelAuthenticated(userId)
	return r, nil
}

func (m *manager) GrantReadAndWriteTokenAccess(ctx context.Context, projectId primitive.ObjectID, epoch int64, userId primitive.ObjectID) error {
	u := &bson.M{
		"$inc": EpochField{Epoch: 1},
		"$addToSet": &bson.M{
			"tokenAccessReadAndWrite_refs": userId,
		},
	}
	return m.setWithEpochGuard(ctx, projectId, epoch, u)
}

func (m *manager) GrantReadOnlyTokenAccess(ctx context.Context, projectId primitive.ObjectID, epoch int64, userId primitive.ObjectID) error {
	u := &bson.M{
		"$inc": EpochField{Epoch: 1},
		"$addToSet": &bson.M{
			"tokenAccessReadOnly_refs": userId,
		},
	}
	return m.setWithEpochGuard(ctx, projectId, epoch, u)
}

func (m *manager) MarkAsActive(ctx context.Context, projectId primitive.ObjectID) error {
	return m.set(ctx, projectId, &ActiveField{Active: true})
}

func (m *manager) MarkAsInActive(ctx context.Context, projectId primitive.ObjectID) error {
	return m.set(ctx, projectId, &ActiveField{Active: false})
}

func (m *manager) MarkAsOpened(ctx context.Context, projectId primitive.ObjectID) error {
	return m.set(ctx, projectId, &LastOpenedField{LastOpened: time.Now()})
}

func (m *manager) RemoveMember(ctx context.Context, projectId primitive.ObjectID, epoch int64, userId primitive.ObjectID) error {
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
	r, err := m.c.DeleteOne(ctx, q)
	if err != nil {
		return rewriteMongoError(err)
	}
	if r.DeletedCount != 1 {
		return ErrVersionChanged
	}
	return nil
}

func (m *manager) Restore(ctx context.Context, p *ForDeletion) error {
	_, err := m.c.InsertOne(ctx, p)
	if err != nil {
		return rewriteMongoError(err)
	}
	return nil
}
