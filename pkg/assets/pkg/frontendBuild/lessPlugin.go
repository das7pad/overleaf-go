// Golang port of Overleaf
// Copyright (C) 2023 Jakob Ackermann <das7pad@outlook.com>
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

package frontendBuild

import (
	"log"
	"path"
	"time"

	"github.com/evanw/esbuild/pkg/api"

	"github.com/das7pad/overleaf-go/pkg/assets/pkg/frontendBuild/pkg/less"
	"github.com/das7pad/overleaf-go/pkg/errors"
)

func lessLoaderPlugin() api.Plugin {
	parse := less.WithCache()
	return api.Plugin{
		Name: "lessLoader",
		Setup: func(build api.PluginBuild) {
			build.OnLoad(api.OnLoadOptions{
				Filter: "\\.less$",
			}, func(args api.OnLoadArgs) (api.OnLoadResult, error) {
				t0 := time.Now()
				s, srcMap, imports, err := parse(args.Path)
				if err != nil {
					return api.OnLoadResult{}, errors.Tag(err, args.Path)
				}
				log.Println(args.Path, "less", time.Since(t0))
				s = less.InlineSourceMap(s, srcMap)
				return api.OnLoadResult{
					ResolveDir: path.Dir(args.Path),
					Contents:   &s,
					WatchFiles: imports,
					WatchDirs:  make([]string, 0),
					Loader:     api.LoaderCSS,
				}, nil
			})
		},
	}
}
