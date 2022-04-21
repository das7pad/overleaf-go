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

package tag

import (
	"github.com/edgedb/edgedb-go"
)

type Full struct {
	IdField       `edgedb:"$inline"`
	NameField     `edgedb:"$inline"`
	ProjectsField `edgedb:"$inline"`
	ProjectIdsField
}

type Tags []Full

func (t Full) Finalize() Full {
	t.ProjectIds = make([]edgedb.UUID, len(t.Projects))
	for _, project := range t.Projects {
		t.ProjectIds = append(t.ProjectIds, project.Id)
	}
	return t
}

func (ts Tags) Finalize() Tags {
	for i, t := range ts {
		ts[i] = t.Finalize()
	}
	return ts
}
