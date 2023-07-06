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

func (o *outputCollector) writeStaticFiles() error {
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
	// OpenInOverleaf
	pattern = append(pattern, staticCopyPattern{
		From: join(o.root, "frontend/js/vendor/libs/highlight.pack.js"),
		To:   join(o.root, "public/vendor/highlight.pack.js"),
	}, staticCopyPattern{
		From: join(o.root, "frontend/stylesheets/vendor/highlight-github.css"),
		To:   join(o.root, "public/vendor/stylesheets/highlight-github.css"),
	})

	wantedPackages := make(map[string]bool)
	for _, cp := range pattern {
		if cp.FromNPMPackage() {
			pkg, _, _ := strings.Cut(cp.From, "/")
			wantedPackages[pkg] = true
		}
	}
	r := yarnPNPReader{}
	defer r.Close()
	if err := r.load(o.root, wantedPackages); err != nil {
		return errors.Tag(err, "load yarn reader")
	}
	for _, cp := range pattern {
		if cp.FromNPMPackage() {
			pkg, prefix, _ := strings.Cut(cp.From, "/")
			for s, file := range r.GetMatching(pkg, prefix) {
				f, err := file.Open()
				if err != nil {
					return errors.Tag(err, s)
				}
				blob, err := io.ReadAll(f)
				if err != nil {
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

	if err := o.writeManifest(); err != nil {
		return err
	}
	if err := o.notifyAboutBuild("static", &api.BuildResult{}); err != nil {
		return err
	}

	return nil
}
