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

	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type DiffMeta struct {
	UserId *primitive.ObjectID                  `json:"user_id,omitempty"`
	User   *user.WithPublicInfoAndNonStandardId `json:"user,omitempty"`

	StartTs sharedTypes.Timestamp `json:"start_ts"`
	EndTs   sharedTypes.Timestamp `json:"end_ts"`
}

type DiffEntry struct {
	Meta *DiffMeta `json:"meta,omitempty"`

	Deletion  sharedTypes.Snippet `json:"d,omitempty"`
	Insertion sharedTypes.Snippet `json:"i,omitempty"`
	Unchanged sharedTypes.Snippet `json:"u,omitempty"`
}

type GetDocDiffRequest struct {
	ProjectId primitive.ObjectID `json:"-"`
	DocId     primitive.ObjectID `json:"-"`
	UserId    primitive.ObjectID `json:"-"`

	From sharedTypes.Version `form:"from" json:"from"`
	To   sharedTypes.Version `form:"to" json:"to"`
}

type GetDocDiffResponse struct {
	Diff []DiffEntry `json:"diff"`
}
