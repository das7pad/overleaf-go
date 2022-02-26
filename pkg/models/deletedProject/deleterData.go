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

package deletedProject

import (
	"time"

	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/models/project"
)

type DeleterDataDeletedProjectIdField struct {
	DeletedProjectId edgedb.UUID `edgedb:"deletedProjectId"`
}

type DeleterData struct {
	DeleterDataDeletedProjectIdField      `edgedb:"inline"`
	DeletedAt                             time.Time           `edgedb:"deletedAt"`
	DeleterId                             edgedb.UUID         `edgedb:"deleterId"`
	DeleterIpAddress                      string              `edgedb:"deleterIpAddress"`
	DeletedProjectOwnerId                 edgedb.UUID         `edgedb:"DeletedProjectOwnerId"`
	DeletedProjectCollaboratorIds         project.Refs        `edgedb:"deletedProjectCollaboratorIds"`
	DeletedProjectReadOnlyIds             project.Refs        `edgedb:"deletedProjectReadOnlyIds"`
	DeletedProjectReadWriteTokenAccessIds project.Refs        `edgedb:"deletedProjectReadWriteTokenAccessIds"`
	DeletedProjectReadOnlyTokenAccessIds  project.Refs        `edgedb:"deletedProjectReadOnlyTokenAccessIds"`
	DeletedProjectReadWriteToken          project.AccessToken `edgedb:"deletedProjectReadWriteToken"`
	DeletedProjectReadOnlyToken           project.AccessToken `edgedb:"deletedProjectReadOnlyToken"`
	DeletedProjectLastUpdatedAt           time.Time           `edgedb:"deletedProjectLastUpdatedAt"`
}

type deleterDataOnlyProjectId struct {
	DeleterDataDeletedProjectIdField `edgedb:"inline"`
}

type forListing struct {
	DeleterData deleterDataOnlyProjectId `edgedb:"deleterData"`
}
