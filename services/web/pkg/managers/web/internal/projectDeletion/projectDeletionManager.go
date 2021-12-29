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

	"go.mongodb.org/mongo-driver/mongo"

	"github.com/das7pad/overleaf-go/pkg/models/deletedFile"
	"github.com/das7pad/overleaf-go/pkg/models/deletedProject"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/tag"
	"github.com/das7pad/overleaf-go/services/chat/pkg/managers/chat"
	"github.com/das7pad/overleaf-go/services/docstore/pkg/managers/docstore"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater"
	"github.com/das7pad/overleaf-go/services/filestore/pkg/managers/filestore"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	DeleteProject(ctx context.Context, request *types.DeleteProjectRequest) error
	UnDeleteProject(ctx context.Context, request *types.UnDeleteProjectRequest) error
	DeleteProjectInTx(ctx, sCtx context.Context, request *types.DeleteProjectRequest) error
	HardDeleteExpiredProjects(ctx context.Context, dryRun bool) error
}

func New(db *mongo.Database, pm project.Manager, tm tag.Manager, cm chat.Manager, dm docstore.Manager, dum documentUpdater.Manager, fm filestore.Manager) Manager {
	return &manager{
		cm:  cm,
		db:  db,
		dfm: deletedFile.New(db),
		dpm: deletedProject.New(db),
		dm:  dm,
		dum: dum,
		fm:  fm,
		pm:  pm,
		tm:  tm,
	}
}

type manager struct {
	cm  chat.Manager
	db  *mongo.Database
	dfm deletedFile.Manager
	dpm deletedProject.Manager
	dm  docstore.Manager
	dum documentUpdater.Manager
	fm  filestore.Manager
	pm  project.Manager
	tm  tag.Manager
}