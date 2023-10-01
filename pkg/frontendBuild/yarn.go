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
	"archive/zip"
	"path"
	"strings"

	"github.com/evanw/esbuild/pkg/api"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

func newYarnPNPReader(root string, build api.PluginBuild, res *api.OnStartResult) yarnPNPReader {
	return yarnPNPReader{
		build: build,
		m:     make(map[string]*zip.ReadCloser),
		root:  root,
		res:   res,
	}
}

type yarnPNPReader struct {
	root  string
	m     map[string]*zip.ReadCloser
	build api.PluginBuild
	res   *api.OnStartResult
}

func (r *yarnPNPReader) getZip(pkg string) (*zip.ReadCloser, error) {
	if z, ok := r.m[pkg]; ok {
		return z, nil
	}
	// We need to resolve a static file in the package to deduce the zip name.
	// Every package has a package.json file at the root, use that.
	probeFile := "package.json"
	res := r.build.Resolve(join(pkg, probeFile), api.ResolveOptions{
		Kind:       api.ResolveJSRequireResolve,
		ResolveDir: r.root,
	})
	if len(res.Errors) > 0 || len(res.Warnings) > 0 {
		r.res.Errors = res.Errors
		r.res.Warnings = res.Warnings
		return nil, errors.New("resolve failed for: " + pkg)
	}
	inZip := join("node_modules", pkg, "/")
	zipName := path.Base(strings.TrimSuffix(res.Path, "/"+inZip+probeFile))
	p := join(r.root, ".yarn/cache", zipName)
	z, err := zip.OpenReader(p)
	if err != nil {
		return nil, errors.Tag(err, "open zip for "+pkg)
	}
	r.m[pkg] = z
	return z, nil
}

func (r *yarnPNPReader) Close() {
	for _, closer := range r.m {
		_ = closer.Close()
	}
}

func (r *yarnPNPReader) GetMatching(pkg, prefix string) (map[string]*zip.File, error) {
	z, err := r.getZip(pkg)
	if err != nil {
		return nil, err
	}
	out := make(map[string]*zip.File)
	inZip := "node_modules/" + pkg + "/"
	for _, file := range z.File {
		if strings.HasPrefix(file.Name, inZip+prefix) {
			out[file.Name[len(inZip)+len(prefix):]] = file
		}
	}
	return out, nil
}
