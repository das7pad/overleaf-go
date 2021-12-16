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

package types

import (
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/doc"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	chatTypes "github.com/das7pad/overleaf-go/services/chat/pkg/types"
)

type AcceptReviewChangesRequest struct {
	ProjectId primitive.ObjectID `json:"-"`
	DocId     primitive.ObjectID `json:"-"`

	ChangeIds []primitive.ObjectID `json:"change_ids"`
}

type DeleteReviewCommentRequest struct {
	ProjectId primitive.ObjectID `json:"-"`
	ThreadId  primitive.ObjectID `json:"-"`
	MessageId primitive.ObjectID `json:"-"`
}

type DeleteReviewThreadRequest struct {
	ProjectId primitive.ObjectID `json:"-"`
	DocId     primitive.ObjectID `json:"-"`
	ThreadId  primitive.ObjectID `json:"-"`
}

type EditReviewCommentRequest struct {
	ProjectId primitive.ObjectID `json:"-"`
	ThreadId  primitive.ObjectID `json:"-"`
	MessageId primitive.ObjectID `json:"-"`

	Content string `bson:"content"`
}

type GetReviewRangesRequest struct {
	ProjectId primitive.ObjectID `json:"-"`
}

type GetReviewRangesResponse = []doc.Ranges

type GetReviewThreadsRequest struct {
	ProjectId primitive.ObjectID `json:"-"`
}

type GetReviewThreadsResponse = chatTypes.Threads

type GetReviewUsersRequest struct {
	ProjectId primitive.ObjectID `json:"-"`
}

type GetReviewUsersResponse = []*user.WithPublicInfoAndNonStandardId

type ReopenReviewThreadRequest struct {
	ProjectId primitive.ObjectID `json:"-"`
	ThreadId  primitive.ObjectID `json:"-"`
}

type ResolveReviewThreadRequest struct {
	ProjectId primitive.ObjectID `json:"-"`
	ThreadId  primitive.ObjectID `json:"-"`
	UserId    primitive.ObjectID `json:"-"`
}

type SendReviewCommentRequest struct {
	ProjectId primitive.ObjectID `json:"-"`
	ThreadId  primitive.ObjectID `json:"-"`
	UserId    primitive.ObjectID `json:"-"`

	Content string `bson:"content"`
}

type SetTrackChangesStateRequest struct {
	ProjectId primitive.ObjectID `json:"-"`

	On          *bool                     `json:"on"`
	OnFor       project.TrackChangesState `json:"on_for"`
	OnForGuests bool                      `json:"on_for_guests"`
}

func (r *SetTrackChangesStateRequest) Validate() error {
	if r.On != nil && len(r.OnFor) > 0 {
		return &errors.ValidationError{
			Msg: "cannot change globally and per-user in same request",
		}
	}
	if r.On == nil && len(r.OnFor) == 0 {
		return &errors.ValidationError{
			Msg: "must enabled/disable globally or per user",
		}
	}
	if err := r.OnFor.Validate(); err != nil {
		return err
	}
	return nil
}
