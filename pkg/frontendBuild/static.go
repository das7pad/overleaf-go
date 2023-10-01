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
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/evanw/esbuild/pkg/api"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type staticCopyPattern struct {
	From string
	To   string
}

func (cp staticCopyPattern) FromNPMPackage() bool {
	return !strings.HasPrefix(cp.From, "/")
}

func join(parts ...string) string {
	p := path.Join(parts...)
	if strings.HasSuffix(parts[len(parts)-1], "/") {
		p += "/"
	}
	return p
}

func (o *outputCollector) copyFile(from, to string) error {
	blob, err := os.ReadFile(from)
	if err != nil {
		return errors.Tag(err, from)
	}
	if err = o.write(to, blob); err != nil {
		return err
	}
	return nil
}

func (o *outputCollector) copyFolder(from, to string) error {
	return filepath.Walk(from, func(file string, s fs.FileInfo, err error) error {
		if err != nil {
			return errors.Tag(err, file)
		}
		if s.IsDir() {
			return nil
		}
		if err = o.copyFile(file, to+file[len(from):]); err != nil {
			return err
		}
		return nil
	})
}

func (o *outputCollector) writeStaticFiles(build api.PluginBuild, res *api.OnStartResult) error {
	var pattern []staticCopyPattern
	pattern = append(pattern, staticCopyPattern{
		From: join(o.root, "LICENSE"),
		To:   join(o.root, "public/LICENSE"),
	})
	// public/
	pattern = append(pattern, staticCopyPattern{
		From: join(o.root, "public/"),
		To:   join(o.root, "public/"),
	})
	// PDF.js
	pattern = append(pattern, staticCopyPattern{
		From: "pdfjs-dist/build/pdf.worker.min.js",
		To:   join(o.root, "public/vendor/pdfjs-dist/build/pdf.worker.min.js"),
	}, staticCopyPattern{
		From: "pdfjs-dist/cmaps/",
		To:   join(o.root, "public/vendor/pdfjs-dist/cmaps/"),
	})
	// Ace
	pattern = append(pattern, staticCopyPattern{
		From: "ace-builds/src-min-noconflict/",
		To:   join(o.root, "public/vendor/ace-builds/src-min-noconflict/"),
	})
	// MathJax
	for _, s := range []string{
		"extensions/a11y/",
		"extensions/HelpDialog.js",
		"fonts/HTML-CSS/TeX/woff/",
		"jax/output/HTML-CSS/autoload/",
		"jax/output/HTML-CSS/fonts/TeX/",
	} {
		pattern = append(pattern, staticCopyPattern{
			From: "mathjax/" + s,
			To:   join(o.root, "public/vendor/mathjax-2-7-9", s),
		})
	}

	r := newYarnPNPReader(o.root, build, res)
	defer r.Close()
	for _, cp := range pattern {
		if cp.FromNPMPackage() {
			pkg, prefix, _ := strings.Cut(cp.From, "/")
			files, err2 := r.GetMatching(pkg, prefix)
			if err2 != nil {
				return err2
			}
			for s, file := range files {
				f, err := file.Open()
				if err != nil {
					return errors.Tag(err, s)
				}
				blob, err := io.ReadAll(f)
				if err != nil {
					_ = f.Close()
					return errors.Tag(err, s)
				}
				if err = f.Close(); err != nil {
					return errors.Tag(err, s)
				}
				if err = o.write(cp.To+s, blob); err != nil {
					return err
				}
			}
		} else {
			if strings.HasSuffix(cp.From, "/") {
				if err := o.copyFolder(cp.From, cp.To); err != nil {
					return err
				}
			} else {
				if err := o.copyFile(cp.From, cp.To); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (o *outputCollector) staticConfig() buildOptions {
	cfg := baseConfig(o.root, "static")
	cfg.Plugins = append(cfg.Plugins, api.Plugin{
		Name: "static",
		Setup: func(build api.PluginBuild) {
			build.OnStart(func() (api.OnStartResult, error) {
				res := api.OnStartResult{}
				err := o.writeStaticFiles(build, &res)
				if len(res.Errors) > 0 {
					res.Errors = append(res.Errors, api.Message{
						Text:   err.Error(),
						Detail: err,
					})
					err = nil
				}
				return res, err
			})
		},
	})
	return cfg
}
