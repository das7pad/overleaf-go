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
	"net/url"

	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	chatTypes "github.com/das7pad/overleaf-go/services/chat/pkg/types"
)

type GetProjectChatMessagesRequest struct {
	ProjectId edgedb.UUID           `form:"-"`
	Before    sharedTypes.Timestamp `form:"before"`
}

func (r *GetProjectChatMessagesRequest) FromQuery(q url.Values) error {
	if err := r.Before.ParseIfSet(q.Get("before")); err != nil {
		return errors.Tag(err, "query parameter 'before'")
	}
	return nil
}

type GetProjectChatMessagesResponse = []chatTypes.Message

type SendProjectChatMessageRequest struct {
	ProjectId edgedb.UUID `json:"-"`
	UserId    edgedb.UUID `json:"-"`
	Content   string      `json:"content"`
}
