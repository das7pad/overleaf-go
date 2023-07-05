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
	"regexp"
	"strings"

	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

//goland:noinspection SpellCheckingInspection
var (
	labelRegex          = regexp.MustCompile(`\\label{([^}]{1,80})}`)
	usePackageRegex     = regexp.MustCompile(`\\usepackage(?:\[[^]]{0,80}?])?{([^}]{1,80})}`)
	requirePackageRegex = regexp.MustCompile(`\\RequirePackage(?:\[[^]]{0,80}?])?{([^}]{1,80})}`)
)

func (m *manager) parseDoc(snapshot string) types.LightDocProjectMetadata {
	s := snapshot
	rawLabels := labelRegex.FindAllStringSubmatch(s, -1)
	rawUsePackages := usePackageRegex.FindAllStringSubmatch(s, -1)
	rawRequirePackages := requirePackageRegex.FindAllStringSubmatch(s, -1)
	n := len(rawUsePackages) + len(rawRequirePackages)
	packageNamesUniq := make(map[types.LatexPackageName]bool, n)
	for _, raw := range [][][]string{rawUsePackages, rawRequirePackages} {
		for _, ps := range raw {
			for _, p := range strings.Split(ps[1], ",") {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				packageNamesUniq[types.LatexPackageName(p)] = true
			}
		}
	}
	packageNames := make([]types.LatexPackageName, 0, len(packageNamesUniq))
	for p := range packageNamesUniq {
		if _, exists := packageMapping[p]; !exists {
			continue
		}
		packageNames = append(packageNames, p)
	}
	labels := make([]types.LatexLabel, len(rawLabels))
	for i, label := range rawLabels {
		labels[i] = types.LatexLabel(label[1])
	}
	return types.LightDocProjectMetadata{
		Labels:       labels,
		PackageNames: packageNames,
	}
}
