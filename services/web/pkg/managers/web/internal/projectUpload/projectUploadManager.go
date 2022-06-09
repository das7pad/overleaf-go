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

package projectUpload

import (
	"context"
	"database/sql"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater"
	"github.com/das7pad/overleaf-go/services/filestore/pkg/managers/filestore"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	CloneProject(ctx context.Context, request *types.CloneProjectRequest, response *types.CloneProjectResponse) error
	CreateProject(ctx context.Context, request *types.CreateProjectRequest, response *types.CreateProjectResponse) error
	CreateExampleProject(ctx context.Context, request *types.CreateExampleProjectRequest, response *types.CreateExampleProjectResponse) error
	CreateFromZip(ctx context.Context, request *types.CreateProjectFromZipRequest, response *types.CreateProjectResponse) error
}

func New(options *types.Options, db *sql.DB, pm project.Manager, um user.Manager, dum documentUpdater.Manager, fm filestore.Manager) Manager {
	return &manager{
		db:      db,
		dum:     dum,
		fm:      fm,
		pm:      pm,
		um:      um,
		options: options,
	}
}

type manager struct {
	db      *sql.DB
	dum     documentUpdater.Manager
	fm      filestore.Manager
	pm      project.Manager
	um      user.Manager
	options *types.Options
}

func (m *manager) purgeFilestoreData(projectId sharedTypes.UUID) error {
	ctx, done := context.WithTimeout(context.Background(), 30*time.Second)
	defer done()

	if err := m.fm.DeleteProject(ctx, projectId); err != nil {
		return errors.Tag(err, "cannot cleanup filestore data")
	}
	return nil
}
