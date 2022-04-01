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
	"encoding/json"
	"time"

	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/models/message"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/pubSub/channel"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/docstore/pkg/managers/docstore"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	AcceptReviewChanges(ctx context.Context, r *types.AcceptReviewChangesRequest) error
	DeleteReviewComment(ctx context.Context, r *types.DeleteReviewCommentRequest) error
	DeleteReviewThread(ctx context.Context, r *types.DeleteReviewThreadRequest) error
	EditReviewComment(ctx context.Context, r *types.EditReviewCommentRequest) error
	GetReviewRanges(ctx context.Context, r *types.GetReviewRangesRequest, response *types.GetReviewRangesResponse) error
	GetReviewThreads(ctx context.Context, r *types.GetReviewThreadsRequest, response *types.GetReviewThreadsResponse) error
	GetReviewUsers(ctx context.Context, r *types.GetReviewUsersRequest, response *types.GetReviewUsersResponse) error
	ReopenReviewThread(ctx context.Context, r *types.ReopenReviewThreadRequest) error
	ResolveReviewThread(ctx context.Context, r *types.ResolveReviewThreadRequest) error
	SendReviewComment(ctx context.Context, r *types.SendReviewCommentRequest) error
	SetTrackChangesState(ctx context.Context, r *types.SetTrackChangesStateRequest) error
}

func New(pm project.Manager, um user.Manager, cm message.Manager, dm docstore.Manager, dum documentUpdater.Manager, editorEvents channel.Writer) Manager {
	return &manager{
		cm:           cm,
		dm:           dm,
		dum:          dum,
		editorEvents: editorEvents,
		um:           um,
		pm:           pm,
	}
}

type manager struct {
	cm           message.Manager
	dm           docstore.Manager
	dum          documentUpdater.Manager
	editorEvents channel.Writer
	pm           project.Manager
	um           user.Manager
}

func (m *manager) notifyEditor(projectId edgedb.UUID, message string, args ...interface{}) {
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	blob, err := json.Marshal(args)
	if err != nil {
		return
	}
	_ = m.editorEvents.Publish(ctx, &sharedTypes.EditorEventsMessage{
		RoomId:  projectId,
		Message: message,
		Payload: blob,
	})
}
