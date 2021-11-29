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

package projectUpload

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"

	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/services/docstore/pkg/managers/docstore"
	"github.com/das7pad/overleaf-go/services/filestore/pkg/managers/filestore"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	CreateFromZip(ctx context.Context, request *types.CreateProjectFromZipRequest, response *types.CreateProjectFromZipResponse) error
}

func New(options *types.Options, db *mongo.Database, pm project.Manager, dm docstore.Manager, fm filestore.Manager) Manager {
	return &manager{
		db:      db,
		dm:      dm,
		fm:      fm,
		pm:      pm,
		options: options,
	}
}

type manager struct {
	db      *mongo.Database
	dm      docstore.Manager
	fm      filestore.Manager
	pm      project.Manager
	options *types.Options
}