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

package main

import (
	"encoding/json"
	"os/exec"
	"path"
	"strings"

	"github.com/evanw/esbuild/pkg/api"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

func lessLoaderPlugin(p string) api.Plugin {
	return api.Plugin{
		Name: "lessLoader",
		Setup: func(build api.PluginBuild) {
			build.OnLoad(api.OnLoadOptions{
				Filter: "\\.less$",
			}, func(args api.OnLoadArgs) (api.OnLoadResult, error) {
				return renderLess(p, args)
			})
		},
	}
}

func renderLess(p string, args api.OnLoadArgs) (api.OnLoadResult, error) {
	c := exec.Command(
		"node",
		"--require", path.Join(p, ".pnp.cjs"),
		path.Join(p, "esbuild/plugins/lessRenderer.js"),
		args.Path,
	)
	blob, err := c.Output()
	if err != nil {
		r := api.OnLoadResult{}
		if e, ok := err.(*exec.ExitError); ok {
			for _, s := range strings.Split(string(e.Stderr), "\n") {
				r.Warnings = append(r.Warnings, api.Message{Text: s})
			}
		}
		return r, errors.Tag(err, args.Path)
	}
	r := api.OnLoadResult{
		Loader: api.LoaderCSS,
	}
	if err = json.Unmarshal(blob, &r); err != nil {
		return api.OnLoadResult{}, errors.Tag(err, args.Path+": parse json")
	}
	return r, nil
}
