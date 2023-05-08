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

package projectMetadata

import (
	_ "embed"
	"encoding/json"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

//go:embed data/packageMapping.json
var _packageMappingRaw []byte
var packageMapping types.LatexPackages

func init() {
	err := json.Unmarshal(_packageMappingRaw, &packageMapping)
	if err != nil {
		panic(errors.Tag(err, "load package metadata mapping"))
	}
	_packageMappingRaw = nil
}

func inflate(in types.LightDocProjectMetadata) types.ProjectDocMetadata {
	packages := make(types.LatexPackages, len(in.PackageNames))
	for _, name := range in.PackageNames {
		packages[name] = packageMapping[name]
	}
	return types.ProjectDocMetadata{
		Labels:   in.Labels,
		Packages: packages,
	}
}
