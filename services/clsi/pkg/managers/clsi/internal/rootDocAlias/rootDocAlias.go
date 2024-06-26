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

package rootDocAlias

import (
	"strings"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

type Manager interface {
	AddAliasDocIfNeeded(request *types.CompileRequest)
}

func New() Manager {
	return &manager{}
}

type manager struct{}

//goland:noinspection SpellCheckingInspection
const (
	usesTikzExternalize = "\\tikzexternalize"
	usesPsTool          = "{pstool}"
	AliasDocFileName    = sharedTypes.PathName("output.tex")
)

func (m *manager) AddAliasDocIfNeeded(request *types.CompileRequest) {
	if request.Options.RootResourcePath == AliasDocFileName {
		// Very fast path
		return
	}
	for _, resource := range request.Resources {
		if resource.Path == AliasDocFileName {
			// Still fast path
			return
		}
	}
	if !strings.Contains(request.RootDoc.Content, usesTikzExternalize) &&
		!strings.Contains(request.RootDoc.Content, usesPsTool) {
		return
	}
	aliasDoc := &types.Resource{
		Path:    AliasDocFileName,
		Content: request.RootDoc.Content,
		Version: request.RootDoc.Version,
	}
	request.Resources = append(request.Resources, aliasDoc)
	request.RootDocAliasResource = aliasDoc
}
