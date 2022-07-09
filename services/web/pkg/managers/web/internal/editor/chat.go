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

package editor

import (
	"context"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/message"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

const chatPageSize = 50

func (m *manager) GetProjectMessages(ctx context.Context, request *types.GetProjectChatMessagesRequest, response *types.GetProjectChatMessagesResponse) error {
	err := m.mm.GetGlobalMessages(
		ctx, request.ProjectId, chatPageSize, request.Before, response,
	)
	res := *response
	for i, msg := range res {
		res[i].User.IdNoUnderscore = msg.User.Id
	}
	return err
}

func (m *manager) SendProjectMessage(ctx context.Context, request *types.SendProjectChatMessageRequest) error {
	msg := message.Message{}
	msg.Content = request.Content
	msg.User.Id = request.UserId
	if err := m.mm.SendGlobalMessage(ctx, request.ProjectId, &msg); err != nil {
		return errors.Tag(err, "cannot persist message")
	}
	msg.User.IdNoUnderscore = msg.User.Id

	go m.notifyEditor(request.ProjectId, "new-chat-message", msg)
	return nil
}
