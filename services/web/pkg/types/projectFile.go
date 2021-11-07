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
	"io"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/session"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type GetProjectFileSizeRequest struct {
	Session   *session.Session   `json:"-"`
	ProjectId primitive.ObjectID `json:"-"`
	FileId    primitive.ObjectID `json:"-"`
}

type GetProjectFileSizeResponse struct {
	Size int64 `json:"-"`
}

type GetProjectFileRequest struct {
	Session   *session.Session   `json:"-"`
	ProjectId primitive.ObjectID `json:"-"`
	FileId    primitive.ObjectID `json:"-"`
}

type GetProjectFileResponse struct {
	Reader   io.Reader            `json:"-"`
	Filename sharedTypes.Filename `json:"-"`
}
