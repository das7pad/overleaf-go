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

package fileTree

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"

	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/pubSub/channel"
	"github.com/das7pad/overleaf-go/services/docstore/pkg/managers/docstore"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	AddDocToProject(ctx context.Context, request *types.AddDocRequest, response *types.AddDocResponse) error
	GetProjectEntities(ctx context.Context, request *types.GetProjectEntitiesRequest, response *types.GetProjectEntitiesResponse) error
}

func New(db *mongo.Database, pm project.Manager, dm docstore.Manager, dum documentUpdater.Manager, editorEvents channel.Writer) Manager {
	return &manager{
		db:           db,
		dm:           dm,
		dum:          dum,
		editorEvents: editorEvents,
		pm:           pm,
	}
}

type manager struct {
	db           *mongo.Database
	dm           docstore.Manager
	dum          documentUpdater.Manager
	editorEvents channel.Writer
	pm           project.Manager
}
