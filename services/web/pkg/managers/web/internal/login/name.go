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

package login

import (
	"context"

	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) SetUserName(ctx context.Context, r *types.SetUserName) error {
	if err := r.Session.CheckIsLoggedIn(); err != nil {
		return err
	}
	u := &user.WithNames{
		FirstNameField: user.FirstNameField{
			FirstName: r.FirstName,
		},
		LastNameField: user.LastNameField{
			LastName: r.LastName,
		},
	}
	if err := m.um.SetUserName(ctx, r.Session.User.Id, u); err != nil {
		return err
	}
	r.Session.User.FirstName = r.FirstName
	r.Session.User.LastName = r.LastName
	return nil
}
