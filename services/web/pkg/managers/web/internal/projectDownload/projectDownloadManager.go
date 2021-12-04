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

package projectDownload

import (
	"context"

	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/services/docstore/pkg/managers/docstore"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater"
	"github.com/das7pad/overleaf-go/services/filestore/pkg/managers/filestore"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	CreateMultiProjectZIP(ctx context.Context, request *types.CreateMultiProjectZIPRequest, response *types.CreateProjectZIPResponse) error
	CreateProjectZIP(ctx context.Context, request *types.CreateProjectZIPRequest, response *types.CreateProjectZIPResponse) error
}

func New(pm project.Manager, dm docstore.Manager, dum documentUpdater.Manager, fm filestore.Manager) Manager {
	return &manager{
		dm:  dm,
		dum: dum,
		fm:  fm,
		pm:  pm,
	}
}

type manager struct {
	dm  docstore.Manager
	dum documentUpdater.Manager
	fm  filestore.Manager
	pm  project.Manager
}
