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
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type ProjectMetadataResponse struct {
	ProjectMetadata ProjectMetadata `json:"projectMeta"`
}
type ProjectDocMetadataRequest struct {
	Broadcast bool `json:"broadcast"`
}

type ProjectDocMetadataResponse struct {
	DocId              sharedTypes.UUID    `json:"docId"`
	ProjectDocMetadata *ProjectDocMetadata `json:"meta"`
}

type SuggestedLatexCommand struct {
	Caption string  `json:"caption"`
	Snippet string  `json:"snippet"`
	Meta    string  `json:"meta"`
	Score   float64 `json:"score"`
}

type LatexLabel string
type LatexPackageName string
type LatexPackages map[LatexPackageName][]SuggestedLatexCommand

type LightDocProjectMetadata struct {
	Labels       []LatexLabel       `json:"labels"`
	PackageNames []LatexPackageName `json:"packageNames"`
}
type ProjectDocMetadata struct {
	Labels   []LatexLabel  `json:"labels"`
	Packages LatexPackages `json:"packages"`
}

// Cannot use ObjectID (bytes array) as key, can only use simple string here.

type LightProjectMetadata map[string]*LightDocProjectMetadata
type ProjectMetadata map[string]*ProjectDocMetadata
