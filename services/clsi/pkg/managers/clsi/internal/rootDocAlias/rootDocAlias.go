// Golang port of the Overleaf clsi service
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

type manager struct {
}

const (
	usesTikzExternalize      = "\\tikzexternalize"
	usesPsTool               = "{pstool}"
	AliasDocFileName         = sharedTypes.FileName("output.tex")
	aliasDocRootResourcePath = types.RootResourcePath(AliasDocFileName)
)

func (m *manager) AddAliasDocIfNeeded(request *types.CompileRequest) {
	if request.RootResourcePath == aliasDocRootResourcePath {
		// Very happy path
		return
	}
	for _, resource := range request.Resources {
		if resource.Path == AliasDocFileName {
			// Still happy path
			return
		}
	}
	blob := string(*request.RootDoc.Content)
	if !strings.Contains(blob, usesTikzExternalize) &&
		!strings.Contains(blob, usesPsTool) {
		return
	}
	aliasDoc := &types.Resource{
		Path:    AliasDocFileName,
		Content: request.RootDoc.Content,
	}
	request.Resources = append(request.Resources, aliasDoc)
	request.RootDocAliasResource = aliasDoc
	return
}
