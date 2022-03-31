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

package review

import (
	"context"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	chatTypes "github.com/das7pad/overleaf-go/services/chat/pkg/types"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) DeleteReviewComment(ctx context.Context, r *types.DeleteReviewCommentRequest) error {
	err := m.cm.DeleteMessage(ctx, r.ProjectId, r.ThreadId, r.MessageId)
	if err != nil {
		return errors.Tag(err, "cannot delete message")
	}
	go m.notifyEditor(
		r.ProjectId, "delete-message", r.ThreadId, r.MessageId,
	)
	return nil
}

func (m *manager) DeleteReviewThread(ctx context.Context, r *types.DeleteReviewThreadRequest) error {
	err := m.dum.DeleteReviewThread(ctx, r.ProjectId, r.DocId, r.ThreadId)
	if err != nil {
		return errors.Tag(err, "cannot delete thread in redis")
	}
	if err = m.cm.DeleteThread(ctx, r.ProjectId, r.ThreadId); err != nil {
		return errors.Tag(err, "cannot delete thread in mongo")
	}
	go m.notifyEditor(r.ProjectId, "delete-thread", r.ThreadId)
	return nil
}

func (m *manager) EditReviewComment(ctx context.Context, r *types.EditReviewCommentRequest) error {
	err := m.cm.EditMessage(
		ctx, r.ProjectId, r.ThreadId, r.MessageId, r.Content,
	)
	if err != nil {
		return errors.Tag(err, "cannot edit message")
	}
	go m.notifyEditor(
		r.ProjectId, "edit-message",
		r.ThreadId, r.MessageId, r.Content,
	)
	return nil
}

func (m *manager) GetReviewThreads(ctx context.Context, r *types.GetReviewThreadsRequest, response *types.GetReviewThreadsResponse) error {
	threads, err := m.cm.GetAllThreads(ctx, r.ProjectId)
	if err != nil {
		return errors.Tag(err, "cannot get threads")
	}
	*response = threads
	return nil
}

func (m *manager) ReopenReviewThread(ctx context.Context, r *types.ReopenReviewThreadRequest) error {
	if err := m.cm.ReopenThread(ctx, r.ProjectId, r.ThreadId); err != nil {
		return errors.Tag(err, "cannot resolve thread")
	}
	go m.notifyEditor(r.ProjectId, "reopen-thread", r.ThreadId)
	return nil
}

func (m *manager) ResolveReviewThread(ctx context.Context, r *types.ResolveReviewThreadRequest) error {
	err := m.cm.ResolveThread(ctx, r.ProjectId, r.ThreadId, r.UserId)
	if err != nil {
		return errors.Tag(err, "cannot resolve thread")
	}
	u := &user.WithPublicInfoAndNonStandardId{}
	if err = m.um.GetUser(ctx, r.UserId, u); err != nil {
		return errors.Tag(err, "cannot get user")
	}
	u.IdNoUnderscore = u.Id

	go m.notifyEditor(r.ProjectId, "resolve-thread", r.ThreadId, u)
	return nil
}

func (m *manager) SendReviewComment(ctx context.Context, r *types.SendReviewCommentRequest) error {
	msg := &chatTypes.Message{}
	msg.Content = r.Content
	msg.User.Id = r.UserId
	err := m.cm.SendThreadMessage(ctx, r.ProjectId, r.ThreadId, msg)
	if err != nil {
		return errors.Tag(err, "cannot create message")
	}
	msg.User.IdNoUnderscore = r.UserId

	go m.notifyEditor(r.ProjectId, "new-comment", r.ThreadId, msg)
	return nil
}
