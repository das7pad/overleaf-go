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

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/models/project"
)

type DeleterData struct {
	DeletedAt                             time.Time           `bson:"deletedAt"`
	DeleterId                             primitive.ObjectID  `bson:"deleterId"`
	DeleterIpAddress                      string              `bson:"deleterIpAddress"`
	DeletedProjectId                      primitive.ObjectID  `bson:"deletedProjectId"`
	DeletedProjectOwnerId                 primitive.ObjectID  `bson:"DeletedProjectOwnerId"`
	DeletedProjectCollaboratorIds         project.Refs        `bson:"deletedProjectCollaboratorIds"`
	DeletedProjectReadOnlyIds             project.Refs        `bson:"deletedProjectReadOnlyIds"`
	DeletedProjectReadWriteTokenAccessIds project.Refs        `bson:"deletedProjectReadWriteTokenAccessIds"`
	DeletedProjectReadOnlyTokenAccessIds  project.Refs        `bson:"deletedProjectReadOnlyTokenAccessIds"`
	DeletedProjectReadWriteToken          project.AccessToken `bson:"deletedProjectReadWriteToken"`
	DeletedProjectReadOnlyToken           project.AccessToken `bson:"deletedProjectReadOnlyToken"`
	DeletedProjectLastUpdatedAt           time.Time           `bson:"deletedProjectLastUpdatedAt"`
}
