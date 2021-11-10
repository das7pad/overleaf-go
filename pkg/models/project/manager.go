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
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Manager interface {
	AddTreeElement(ctx context.Context, projectId primitive.ObjectID, version sharedTypes.Version, mongoPath MongoPath, element TreeElement) error
	GetAuthorizationDetails(ctx context.Context, projectId, userId primitive.ObjectID, token AccessToken) (*AuthorizationDetails, error)
	GetEpoch(ctx context.Context, projectId primitive.ObjectID) (int64, error)
	GetDocMeta(ctx context.Context, projectId, docId primitive.ObjectID) (*Doc, sharedTypes.PathName, error)
	GetJoinProjectDetails(ctx context.Context, projectId, userId primitive.ObjectID) (*JoinProjectViewPrivate, error)
	GetLoadEditorDetails(ctx context.Context, projectId, userId primitive.ObjectID) (*LoadEditorViewPrivate, error)
	GetProjectRootFolder(ctx context.Context, projectId primitive.ObjectID) (*Folder, error)
	GetProject(ctx context.Context, projectId primitive.ObjectID, target interface{}) error
	GetProjectAccessForReadAndWriteToken(ctx context.Context, userId primitive.ObjectID, token AccessToken) (*TokenAccessResult, error)
	GetProjectAccessForReadOnlyToken(ctx context.Context, userId primitive.ObjectID, token AccessToken) (*TokenAccessResult, error)
	GetTreeAndAuth(ctx context.Context, projectId, userId primitive.ObjectID) (*WithTreeAndAuth, error)
	GrantReadAndWriteTokenAccess(ctx context.Context, projectId, userId primitive.ObjectID) error
	GrantReadOnlyTokenAccess(ctx context.Context, projectId, userId primitive.ObjectID) error
	ListProjects(ctx context.Context, userId primitive.ObjectID) ([]ListViewPrivate, error)
	MarkAsActive(ctx context.Context, projectId primitive.ObjectID) error
	MarkAsInActive(ctx context.Context, projectId primitive.ObjectID) error
	MarkAsOpened(ctx context.Context, projectId primitive.ObjectID) error
	UpdateLastUpdated(ctx context.Context, projectId primitive.ObjectID, at time.Time, by primitive.ObjectID) error
	ArchiveForUser(ctx context.Context, projectId, userId primitive.ObjectID) error
	UnArchiveForUser(ctx context.Context, projectId, userId primitive.ObjectID) error
	TrashForUser(ctx context.Context, projectId, userId primitive.ObjectID) error
	UnTrashForUser(ctx context.Context, projectId, userId primitive.ObjectID) error
}

func New(db *mongo.Database) Manager {
	return &manager{
		c: db.Collection("projects"),
	}
}

func rewriteMongoError(err error) error {
	if err == mongo.ErrNoDocuments {
		return &errors.ErrorDocNotFound{}
	}
	return err
}

type manager struct {
	c *mongo.Collection
}

func (m *manager) AddTreeElement(ctx context.Context, projectId primitive.ObjectID, version sharedTypes.Version, mongoPath MongoPath, element TreeElement) error {
	q := &withIdAndVersion{}
	q.Id = projectId
	q.Version = version

	u := &bson.M{
		"$push": bson.M{
			string(mongoPath): element,
		},
		"$inc": VersionField{Version: 1},
	}

	r, err := m.c.UpdateOne(ctx, q, u)
	if err != nil {
		return rewriteMongoError(err)
	}
	if r.MatchedCount != 1 {
		return &errors.InvalidStateError{Msg: "project version changed"}
	}
	return nil
}

func (m *manager) ArchiveForUser(ctx context.Context, projectId, userId primitive.ObjectID) error {
	return m.checkAccessAndUpdate(
		ctx, projectId, userId, PrivilegeLevelReadOnly, &bson.M{
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
		ctx, projectId, userId, PrivilegeLevelReadOnly, &bson.M{
			"$pull": bson.M{
				"archived": userId,
			},
		},
	)
}

func (m *manager) TrashForUser(ctx context.Context, projectId, userId primitive.ObjectID) error {
	return m.checkAccessAndUpdate(
		ctx, projectId, userId, PrivilegeLevelReadOnly, &bson.M{
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
		ctx, projectId, userId, PrivilegeLevelReadOnly, &bson.M{
			"$pull": bson.M{
				"trashed": userId,
			},
		},
	)
}

var ErrEpochIsNotStable = errors.New("epoch is not stable")

func (m *manager) checkAccessAndUpdate(ctx context.Context, projectId, userId primitive.ObjectID, minLevel PrivilegeLevel, u interface{}) error {
	for i := 0; i < 10; i++ {
		p := &ForAuthorizationDetails{}
		qId := &IdField{Id: projectId}
		err := m.fetchWithMinimalAuthorizationDetails(ctx, qId, userId, p)
		if err != nil {
			return err
		}
		d, err := p.GetPrivilegeLevelAuthenticated(userId)
		if err != nil {
			return err
		}
		if err = d.PrivilegeLevel.CheckIsAtLeast(minLevel); err != nil {
			return err
		}
		withEpochGuard := withIdAndEpoch{}
		withEpochGuard.Id = projectId
		withEpochGuard.Epoch = d.Epoch
		r, err := m.c.UpdateOne(ctx, withEpochGuard, u)
		if err != nil {
			return err
		}
		if r.MatchedCount != 1 {
			continue
		}
		return nil
	}
	return ErrEpochIsNotStable
}

func (m *manager) ListProjects(ctx context.Context, userId primitive.ObjectID) ([]ListViewPrivate, error) {
	var projects []ListViewPrivate
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

	//goland:noinspection SpellCheckingInspection
	q := bson.M{
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
	f, err := m.GetProjectRootFolder(ctx, projectId)
	if err != nil {
		return nil, "", errors.Tag(err, "cannot get tree")
	}
	var doc *Doc
	var p sharedTypes.PathName
	err = f.WalkDocs(func(element TreeElement, path sharedTypes.PathName) error {
		if element.GetId() == docId {
			d := element.(Doc)
			doc = &d
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

func (m *manager) GetProjectRootFolder(ctx context.Context, projectId primitive.ObjectID) (*Folder, error) {
	var project WithTree
	err := m.c.FindOne(
		ctx,
		bson.M{
			"_id": projectId,
		},
		options.FindOne().SetProjection(getProjection(project)),
	).Decode(&project)
	if err != nil {
		return nil, rewriteMongoError(err)
	}
	return project.GetRootFolder()
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
		Fresh:     freshAccess,
	}
	if userId.IsZero() {
		return r, nil
	}
	r.Existing, _ = p.GetPrivilegeLevelAuthenticated(userId)
	return r, nil
}

func (m *manager) GrantReadAndWriteTokenAccess(ctx context.Context, projectId, userId primitive.ObjectID) error {
	q := &IdField{Id: projectId}
	u := &bson.M{
		"$addToSet": &bson.M{
			"tokenAccessReadAndWrite_refs": userId,
		},
	}
	if _, err := m.c.UpdateOne(ctx, q, u); err != nil {
		return rewriteMongoError(err)
	}
	return nil
}

func (m *manager) GrantReadOnlyTokenAccess(ctx context.Context, projectId, userId primitive.ObjectID) error {
	q := &IdField{Id: projectId}
	u := &bson.M{
		"$addToSet": &bson.M{
			"tokenAccessReadOnly_refs": userId,
		},
	}
	if _, err := m.c.UpdateOne(ctx, q, u); err != nil {
		return rewriteMongoError(err)
	}
	return nil
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
