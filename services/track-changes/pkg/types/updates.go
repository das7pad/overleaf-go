// Golang port of Overleaf
// Copyright (C) 2021-2023 Jakob Ackermann <das7pad@outlook.com>
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

type DocUpdateBounds struct {
	FromV sharedTypes.Version `json:"fromV"`
	ToV   sharedTypes.Version `json:"toV"`
}

type DocUpdateMeta struct {
	UserIds []sharedTypes.UUID                    `json:"user_ids,omitempty"`
	Users   []user.WithPublicInfoAndNonStandardId `json:"users,omitempty"`
	StartTS sharedTypes.Timestamp                 `json:"start_ts"`
	EndTS   sharedTypes.Timestamp                 `json:"end_ts"`
}

type Update struct {
	Meta DocUpdateMeta              `json:"meta"`
	Docs map[string]DocUpdateBounds `json:"docs"`
}

type GetProjectHistoryUpdatesRequest struct {
	ProjectId sharedTypes.UUID `json:"-"`
	UserId    sharedTypes.UUID `json:"-"`

	Before sharedTypes.Timestamp `form:"before" json:"before"`
}

func (r *GetProjectHistoryUpdatesRequest) FromSignedProjectOptions(o sharedTypes.ProjectOptions) {
	r.ProjectId = o.ProjectId
	r.UserId = o.UserId
}

func (r *GetProjectHistoryUpdatesRequest) FromQuery(q url.Values) error {
	if err := r.Before.ParseIfSet(q.Get("before")); err != nil {
		return errors.Tag(err, "query parameter 'before'")
	}
	return nil
}

type GetProjectHistoryUpdatesResponse struct {
	Updates             []Update              `json:"updates"`
	NextBeforeTimestamp sharedTypes.Timestamp `json:"nextBeforeTimestamp,omitempty"`
}
