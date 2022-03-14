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

package user

import (
	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Features struct {
	Collaborators       int                      `json:"collaborators"`
	Versioning          bool                     `json:"versioning"`
	CompileTimeout      edgedb.Duration          `json:"compileTimeout" edgedb:"compile_timeout"`
	CompileGroup        sharedTypes.CompileGroup `json:"compileGroup" edgedb:"compile_group"`
	TrackChanges        bool                     `json:"trackChanges"`
	TrackChangesVisible bool                     `json:"trackChangesVisible"`
}
