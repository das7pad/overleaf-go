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

	"github.com/das7pad/overleaf-go/pkg/models/tag"
	"github.com/das7pad/overleaf-go/pkg/session"
)

type AddProjectToTagRequest struct {
	Session   *session.Session `json:"-"`
	ProjectId edgedb.UUID      `json:"-"`
	TagId     edgedb.UUID      `json:"-"`
}

type CreateTagRequest struct {
	Session *session.Session `json:"-"`
	Name    string           `json:"name"`
}

type CreateTagResponse = tag.Full

type DeleteTagRequest struct {
	Session *session.Session `json:"-"`
	TagId   edgedb.UUID      `json:"-"`
}

type RemoveProjectToTagRequest struct {
	Session   *session.Session `json:"-"`
	ProjectId edgedb.UUID      `json:"-"`
	TagId     edgedb.UUID      `json:"-"`
}

type RenameTagRequest struct {
	Session *session.Session `json:"-"`
	TagId   edgedb.UUID      `json:"-"`
	Name    string           `json:"name"`
}
