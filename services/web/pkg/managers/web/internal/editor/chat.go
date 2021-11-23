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
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) GetProjectMessages(ctx context.Context, request *types.GetProjectChatMessagesRequest, response *types.GetProjectChatMessagesResponse) error {
	rawMessages, err := m.cm.GetGlobalMessages(ctx, request.ProjectId, request.Limit, request.Before)
	if err != nil {
		return errors.Tag(err, "cannot get messages")
	}
	usersUniq := make(user.UniqUserIds, len(rawMessages))
	for _, message := range rawMessages {
		usersUniq[message.UserId] = true
	}
	messages := make([]*types.ChatMessage, len(rawMessages))
	for i, message := range rawMessages {
		messages[i] = &types.ChatMessage{Message: message}
	}
	if len(usersUniq) != 0 {
		users, err2 := m.um.GetUsersForBackFillingNonStandardId(ctx, usersUniq)
		if err2 != nil {
			return errors.Tag(err2, "cannot get user details")
		}
		for _, message := range messages {
			message.User = users[message.UserId]
		}
	}
	*response = messages
	return nil
}

func (m *manager) SendProjectMessage(ctx context.Context, request *types.SendProjectChatMessageRequest) error {
	rawMessage, err := m.cm.SendGlobalMessage(ctx, request.ProjectId, request.Content, request.UserId)
	if err != nil {
		return errors.Tag(err, "cannot persist message")
	}

	// NOTE: Silently bail out when the broadcast fails.
	//       The message has been persisted and re-sending would result in
	//        duplicate messages in the DB.
	u := &user.WithPublicInfoAndNonStandardId{}
	if err = m.um.GetUser(ctx, request.UserId, u); err != nil {
		return nil
	}
	u.IdNoUnderscore = u.Id

	message := types.ChatMessage{
		Message: *rawMessage,
		User:    u,
	}
	go m.notifyEditor(
		request.ProjectId, "new-chat-message", message,
	)
	return nil
}
