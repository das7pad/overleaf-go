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
	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/doc"
	"github.com/das7pad/overleaf-go/pkg/models/message"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/user"
)

type AcceptReviewChangesRequest struct {
	ProjectId edgedb.UUID `json:"-"`
	DocId     edgedb.UUID `json:"-"`

	ChangeIds []edgedb.UUID `json:"change_ids"`
}

type DeleteReviewCommentRequest struct {
	ProjectId edgedb.UUID `json:"-"`
	ThreadId  edgedb.UUID `json:"-"`
	MessageId edgedb.UUID `json:"-"`
}

type DeleteReviewThreadRequest struct {
	ProjectId edgedb.UUID `json:"-"`
	DocId     edgedb.UUID `json:"-"`
	ThreadId  edgedb.UUID `json:"-"`
}

type EditReviewCommentRequest struct {
	ProjectId edgedb.UUID `json:"-"`
	ThreadId  edgedb.UUID `json:"-"`
	MessageId edgedb.UUID `json:"-"`

	Content string `json:"content"`
}

type GetReviewRangesRequest struct {
	ProjectId edgedb.UUID `json:"-"`
}

type GetReviewRangesResponse = []doc.Ranges

type GetReviewThreadsRequest struct {
	ProjectId edgedb.UUID `json:"-"`
}

type GetReviewThreadsResponse = message.Threads

type GetReviewUsersRequest struct {
	ProjectId edgedb.UUID `json:"-"`
}

type GetReviewUsersResponse = []*user.WithPublicInfoAndNonStandardId

type ReopenReviewThreadRequest struct {
	ProjectId edgedb.UUID `json:"-"`
	ThreadId  edgedb.UUID `json:"-"`
}

type ResolveReviewThreadRequest struct {
	ProjectId edgedb.UUID `json:"-"`
	ThreadId  edgedb.UUID `json:"-"`
	UserId    edgedb.UUID `json:"-"`
}

type SendReviewCommentRequest struct {
	ProjectId edgedb.UUID `json:"-"`
	ThreadId  edgedb.UUID `json:"-"`
	UserId    edgedb.UUID `json:"-"`

	Content string `json:"content"`
}

type SetTrackChangesStateRequest struct {
	ProjectId edgedb.UUID `json:"-"`
	UserId    edgedb.UUID `json:"-"`

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
