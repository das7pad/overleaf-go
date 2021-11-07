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

package editor

import (
	"context"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/services/contacts/pkg/managers/contacts"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) GetUserContacts(ctx context.Context, request *types.GetUserContactsRequest, response *types.GetUserContactsResponse) error {
	if err := request.Session.CheckIsLoggedIn(); err != nil {
		return err
	}
	userId := request.Session.User.Id
	sortedIds, err := m.csm.GetContacts(ctx, userId, contacts.DefaultLimit)
	if err != nil {
		return errors.Tag(err, "cannot get contacts")
	}
	userIds := make(user.UniqUserIds, len(sortedIds))
	for _, id := range sortedIds {
		userIds[id] = true
	}
	users, err := m.um.GetUsersForBackFillingNonStandardId(ctx, userIds)
	if err != nil {
		return errors.Tag(err, "cannot get users")
	}
	userContacts := make([]*types.UserContact, 0, len(users))
	for _, id := range sortedIds {
		usr, exists := users[id]
		if !exists {
			continue
		}
		userContacts = append(userContacts, &types.UserContact{
			WithPublicInfoAndNonStandardId: usr,
			Type:                           "user",
		})
	}
	response.Contacts = userContacts
	return nil
}
