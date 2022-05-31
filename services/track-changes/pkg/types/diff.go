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

package types

import (
	"net/url"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type DiffMeta struct {
	User user.WithPublicInfoAndNonStandardId `json:"user,omitempty"`

	StartTs sharedTypes.Timestamp `json:"start_ts"`
	EndTs   sharedTypes.Timestamp `json:"end_ts"`
}

type DiffEntry struct {
	Meta DiffMeta `json:"meta,omitempty"`

	Deletion  sharedTypes.Snippet `json:"d,omitempty"`
	Insertion sharedTypes.Snippet `json:"i,omitempty"`
	Unchanged sharedTypes.Snippet `json:"u,omitempty"`
}

type GetDocDiffRequest struct {
	ProjectId sharedTypes.UUID `json:"-"`
	DocId     sharedTypes.UUID `json:"-"`
	UserId    sharedTypes.UUID `json:"-"`

	From sharedTypes.Version `form:"from" json:"from"`
	To   sharedTypes.Version `form:"to" json:"to"`
}

func (r *GetDocDiffRequest) Validate() error {
	if r.To < r.From {
		return &errors.ValidationError{Msg: "from/to flipped"}
	}
	return nil
}

func (r *GetDocDiffRequest) FromQuery(q url.Values) error {
	if err := r.From.ParseIfSet(q.Get("from")); err != nil {
		return errors.Tag(err, "query parameter 'from'")
	}
	if err := r.To.ParseIfSet(q.Get("to")); err != nil {
		return errors.Tag(err, "query parameter 'to'")
	}
	return nil
}

type GetDocDiffResponse struct {
	Diff []DiffEntry `json:"diff"`
}
