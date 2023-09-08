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
	"path"

	"github.com/evanw/esbuild/pkg/api"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/translations/pkg/translationsImport"
)

func localesLoaderPlugin(root string) api.Plugin {
	c := translationsImport.Cache{
		SourceDirs: []string{path.Join(root, "frontend/js/")},
	}

	return api.Plugin{
		Name: "localesLoader",
		Setup: func(build api.PluginBuild) {
			build.OnLoad(api.OnLoadOptions{
				Filter: "locales/\\w+\\.json$",
			}, func(args api.OnLoadArgs) (api.OnLoadResult, error) {
				s, watch, err := c.ImportLng(
					args.Path,
					func(key string, v string) string {
						return v
					},
				)
				if err != nil {
					return api.OnLoadResult{
						WatchFiles: watch,
						WatchDirs:  make([]string, 0),
					}, errors.Tag(err, args.Path)
				}
				return api.OnLoadResult{
					Contents:   &s,
					WatchFiles: watch,
					WatchDirs:  make([]string, 0),
					Loader:     api.LoaderJSON,
				}, nil
			})
		},
	}
}
