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
	"github.com/das7pad/overleaf-go/pkg/models/tag"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type AddProjectToTagRequest struct {
	WithSession
	ProjectId sharedTypes.UUID `json:"-"`
	TagId     sharedTypes.UUID `json:"-"`
}

type AddProjectsToTagRequest struct {
	WithSession
	ProjectIds []sharedTypes.UUID `json:"projectIds"`
	TagId      sharedTypes.UUID   `json:"-"`
}

type CreateTagRequest struct {
	WithSession
	Name string `json:"name"`
}

type CreateTagResponse = tag.Full

type DeleteTagRequest struct {
	WithSession
	TagId sharedTypes.UUID `json:"-"`
}

type RemoveProjectToTagRequest struct {
	WithSession
	ProjectId sharedTypes.UUID `json:"-"`
	TagId     sharedTypes.UUID `json:"-"`
}

type RemoveProjectsToTagRequest struct {
	WithSession
	ProjectIds []sharedTypes.UUID `json:"projectIds"`
	TagId      sharedTypes.UUID   `json:"-"`
}

type RenameTagRequest struct {
	WithSession
	TagId sharedTypes.UUID `json:"-"`
	Name  string           `json:"name"`
}
