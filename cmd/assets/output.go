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
	"archive/tar"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sync"

	"github.com/evanw/esbuild/pkg/api"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

func newOutputCollector(p string, w io.Writer) *outputCollector {
	o := outputCollector{p: path.Join(p, "public")}
	o.manifest.Assets = make(map[string]string)
	o.manifest.EntryPoints = make(map[string][]string)
	if w != nil {
		o.o = tar.NewWriter(w)
	} else {
		o.oo = make(map[string][]byte)
	}
	return &o
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

type outputCollector struct {
	manifest struct {
		Assets      map[string]string   `json:"assets"`
		EntryPoints map[string][]string `json:"entryPoints"`
	}
	mu sync.Mutex
	p  string

	previous map[string]interface{}

	o  *tar.Writer
	oo map[string][]byte
}

func (o *outputCollector) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.o != nil {
		blob, err := json.Marshal(o.manifest)
		if err != nil {
			return errors.Tag(err, "serialize manifest")
		}
		file := path.Join(o.p, "public/manifest.json")
		if err = o.write(file, blob); err != nil {
			return err
		}
		if err = o.o.Close(); err != nil {
			return errors.Tag(err, "close tar")
		}
	}
	return nil
}

func (o *outputCollector) Plugin(options buildOptions) api.Plugin {
	return api.Plugin{
		Name: "output",
		Setup: func(build api.PluginBuild) {
			build.OnEnd(func(r *api.BuildResult) (api.OnEndResult, error) {
				return o.handleOnEnd(options.Description, r)
			})
		},
	}
}

func (o *outputCollector) append(p string, blob []byte) error {
	err := o.o.WriteHeader(&tar.Header{
		Name: p,
		Size: int64(len(blob)),
		Mode: 0o444,
	})
	if err != nil {
		return errors.Tag(err, p+": write tar header")
	}
	if _, err = o.o.Write(blob); err != nil {
		return errors.Tag(err, p+": write tar body")
	}
	return nil
}

func (o *outputCollector) write(p string, blob []byte) error {
	p = "." + p[len(o.p):]
	if o.o == nil {
		o.oo[p] = blob
		return nil
	}

	if err := o.append(p, blob); err != nil {
		return err
	}

	gz, err := compress(blob)
	if err != nil {
		return errors.Tag(err, p)
	}
	if len(gz) < len(blob) {
		return o.append(p+".gz", gz)
	}
	return nil
}

type manifest struct {
	Outputs map[string]struct {
		Inputs     map[string]struct{}
		Entrypoint string
		CssBundle  string
		Imports    []struct {
			Kind string
			Path string
		}
	}
}

func (o *outputCollector) handleOnEnd(desc string, r *api.BuildResult) (api.OnEndResult, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// TODO track previous

	m := manifest{}
	if err := json.Unmarshal([]byte(r.Metafile), &m); err != nil {
		return api.OnEndResult{}, errors.Tag(err, "deserialize metafile")
	}

	for s, file := range m.Outputs {
		ext := filepath.Ext(s)
		switch ext {
		case ".woff", ".woff2", ".png", ".svg", ".gif":
			for s2 := range file.Inputs {
				o.manifest.Assets[s2] = s[len("public"):]
			}
		}

		bundle := file.Entrypoint
		if bundle == "" {
			continue
		}
		o.manifest.Assets[bundle] = s[len("public"):]

		var e []string
		for _, i := range file.Imports {
			if i.Kind == "import-statement" {
				e = append(e, i.Path[len("public"):])
			}
		}
		e = append(e, s[len("public"):])
		o.manifest.EntryPoints[bundle] = e

		if file.CssBundle != "" {
			o.manifest.Assets[bundle+".css"] = file.CssBundle[len("public"):]
		}
	}

	for _, file := range r.OutputFiles {
		if err := o.write(file.Path, file.Contents); err != nil {
			return api.OnEndResult{}, err
		}
	}

	return api.OnEndResult{}, nil
}
