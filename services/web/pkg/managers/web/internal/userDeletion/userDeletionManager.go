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

package userDeletion

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"

	"github.com/das7pad/overleaf-go/pkg/models/contact"
	"github.com/das7pad/overleaf-go/pkg/models/deletedUser"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/projectDeletion"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	DeleteUser(ctx context.Context, request *types.DeleteUserRequest) error
	HardDeleteExpiredUsers(ctx context.Context, dryRun bool) error
}

func New(db *mongo.Database, pm project.Manager, um user.Manager, cm contact.Manager, pDelM projectDeletion.Manager) Manager {
	return &manager{
		cm:    cm,
		db:    db,
		delUM: deletedUser.New(db),
		pm:    pm,
		um:    um,
		pDelM: pDelM,
	}
}

type manager struct {
	cm    contact.Manager
	db    *mongo.Database
	delUM deletedUser.Manager
	pm    project.Manager
	um    user.Manager
	pDelM projectDeletion.Manager
}
