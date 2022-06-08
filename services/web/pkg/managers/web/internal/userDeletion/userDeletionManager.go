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

package userDeletion

import (
	"context"
	"time"

	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/projectDeletion"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	DeleteUser(ctx context.Context, request *types.DeleteUserRequest) error
	HardDeleteExpiredUsers(ctx context.Context, dryRun bool, start time.Time) error
}

func New(um user.Manager, pDelM projectDeletion.Manager) Manager {
	return &manager{
		um:    um,
		pDelM: pDelM,
	}
}

type manager struct {
	um    user.Manager
	pDelM projectDeletion.Manager
}
