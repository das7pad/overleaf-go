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

package projectDeletion

import (
	"context"
	"log"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/deletedProject"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/tag"
	"github.com/das7pad/overleaf-go/pkg/mongoTx"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/docstore/pkg/managers/docstore"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater"
	"github.com/das7pad/overleaf-go/services/filestore/pkg/managers/filestore"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	DeleteProject(ctx context.Context, request *types.DeleteProjectRequest) error
}

func New(db *mongo.Database, pm project.Manager, tm tag.Manager, dm docstore.Manager, dum documentUpdater.Manager, fm filestore.Manager) Manager {
	return &manager{
		db:  db,
		dpm: deletedProject.New(db),
		dm:  dm,
		dum: dum,
		fm:  fm,
		pm:  pm,
		tm:  tm,
	}
}

type manager struct {
	db  *mongo.Database
	dpm deletedProject.Manager
	dm  docstore.Manager
	dum documentUpdater.Manager
	fm  filestore.Manager
	pm  project.Manager
	tm  tag.Manager
}

func getAllUserIds(p *project.ForDeletion) []primitive.ObjectID {
	n := 1 +
		len(p.CollaboratorRefs) +
		len(p.ReadOnlyRefs) +
		len(p.TokenAccessReadAndWriteRefs) +
		len(p.TokenAccessReadOnlyRefs)
	userIds := make([]primitive.ObjectID, n)
	userIds[0] = p.OwnerRef
	i := 1
	i += copy(userIds[i:], p.CollaboratorRefs)
	i += copy(userIds[i:], p.ReadOnlyRefs)
	i += copy(userIds[i:], p.TokenAccessReadAndWriteRefs)
	i += copy(userIds[i:], p.TokenAccessReadOnlyRefs)
	return userIds
}

func (m *manager) DeleteProject(ctx context.Context, request *types.DeleteProjectRequest) error {
	if err := request.Session.CheckIsLoggedIn(); err != nil {
		return err
	}
	ipAddress := request.IPAddress
	userId := request.Session.User.Id
	projectId := request.ProjectId

	return mongoTx.For(m.db, ctx, func(sCtx context.Context) error {
		p := &project.ForDeletion{}
		if err := m.pm.GetProject(sCtx, projectId, p); err != nil {
			return errors.Tag(err, "cannot get project")
		}
		errAuth := p.CheckPrivilegeLevelIsAtLest(
			userId, sharedTypes.PrivilegeLevelOwner,
		)
		if errAuth != nil {
			return errAuth
		}
		if err := m.dum.FlushAndDeleteProject(ctx, projectId); err != nil {
			return errors.Tag(err, "cannot flush project")
		}
		if err := m.dm.ArchiveProject(ctx, projectId); err != nil {
			// NOTE: Archiving the docs here is an optimization.
			//       They will get deleted from both mongo/s3 on hard deletion.
			err = errors.Tag(err, "cannot archive project")
			log.Printf("%s: %s", projectId.Hex(), err)
		}

		if err := m.dpm.Create(sCtx, p, userId, ipAddress); err != nil {
			return errors.Tag(err, "cannot create deleted project")
		}

		allUserIds := getAllUserIds(p)
		errTags := m.tm.RemoveProjectForAllUsers(sCtx, allUserIds, projectId)
		if errTags != nil {
			return errors.Tag(errTags, "cannot remove project from tags")
		}
		if err := m.pm.Delete(sCtx, p); err != nil {
			return errors.Tag(err, "cannot delete project")
		}
		return nil
	})
}
