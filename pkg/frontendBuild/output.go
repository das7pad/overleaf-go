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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/evanw/esbuild/pkg/api"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type API interface {
	http.Handler
	Build(concurrency int, watch bool) error
	Bundle(w io.Writer) error
	AddListener(c chan BuildNotification) func()
	Get(p string) ([]byte, bool)
}

func NewOutputCollector(root string, preCompress PreCompress) API {
	return &outputCollector{
		manifest: manifest{
			Assets:           make(map[string]string),
			EntrypointChunks: make(map[string][]string),
		},
		mem:         make(map[string][]byte),
		old:         make(map[string]map[string]uint8),
		root:        root,
		preCompress: preCompress,
	}
}

type PreCompress int

const (
	PreCompressNone = PreCompress(iota)
	PreCompressSource
	PreCompressSourcePlusMap
)

type manifest struct {
	Assets           map[string]string   `json:"assets"`
	EntrypointChunks map[string][]string `json:"entrypointChunks"`
}

type outputCollector struct {
	manifest
	mu          sync.Mutex
	root        string
	preCompress PreCompress

	onBuild []chan<- BuildNotification
	old     map[string]map[string]uint8
	mem     map[string][]byte
}

func (o *outputCollector) writeManifest() error {
	o.mu.Lock()
	blob, err := json.Marshal(o.manifest)
	o.mu.Unlock()
	if err != nil {
		return errors.Tag(err, "serialize manifest")
	}
	if err = o.write(join(o.root, "public/manifest.json"), blob); err != nil {
		return err
	}
	return nil
}

func (o *outputCollector) Bundle(w io.Writer) error {
	gz, errGz := gzip.NewWriterLevel(w, 6)
	if errGz != nil {
		return errGz
	}
	t := tar.NewWriter(gz)

	o.mu.Lock()
	defer o.mu.Unlock()
	ordered := make([]string, 0, len(o.mem))
	for f := range o.mem {
		ordered = append(ordered, f)
	}
	sort.Strings(ordered)
	for _, f := range ordered {
		mode := int64(0o444)
		if strings.HasSuffix(f, "/") {
			mode = 0o755
		}
		err := t.WriteHeader(&tar.Header{
			Name: f,
			Size: int64(len(o.mem[f])),
			Mode: mode,
		})
		if err != nil {
			return errors.Tag(err, f+": write tar header")
		}
		if _, err = t.Write(o.mem[f]); err != nil {
			return errors.Tag(err, f+": write tar body")
		}
	}

	if err := t.Close(); err != nil {
		return errors.Tag(err, "close tar")
	}
	if err := gz.Close(); err != nil {
		return errors.Tag(err, "close gzip")
	}
	return nil
}

func (o *outputCollector) plugin(options buildOptions, firstBuild chan struct{}) api.Plugin {
	return api.Plugin{
		Name: "output",
		Setup: func(build api.PluginBuild) {
			var t0 time.Time
			build.OnStart(func() (api.OnStartResult, error) {
				t0 = time.Now()
				return api.OnStartResult{}, nil
			})
			build.OnEnd(func(r *api.BuildResult) (api.OnEndResult, error) {
				err := o.handleOnEnd(options.Description, r)
				if firstBuild != nil {
					close(firstBuild)
					firstBuild = nil
				}
				log.Println(options.Description, time.Since(t0))
				return api.OnEndResult{}, err
			})
		},
	}
}

func (o *outputCollector) write(p string, blob []byte) error {
	p = p[len(join(o.root, "public"))+1:]
	o.mu.Lock()
	if existing, ok := o.mem[p]; ok && bytes.Equal(blob, existing) {
		o.mu.Unlock()
		return nil
	}
	o.mem[p] = blob
	o.mu.Unlock()

	switch o.preCompress {
	case PreCompressNone:
		return nil
	case PreCompressSource:
		if strings.HasSuffix(p, ".map") {
			return nil
		}
	case PreCompressSourcePlusMap:
	}
	gz, err := compress(blob)
	if err != nil {
		return errors.Tag(err, p)
	}
	if len(gz) < len(blob) {
		o.mu.Lock()
		o.mem[p+".gz"] = gz
		o.mu.Unlock()
	}
	return nil
}

type rawManifest struct {
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

func (o *outputCollector) handleOnEnd(desc string, r *api.BuildResult) error {
	m := rawManifest{}
	if r.Metafile != "" {
		if err := json.Unmarshal([]byte(r.Metafile), &m); err != nil {
			return errors.Tag(err, "deserialize metafile")
		}
	}

	o.mu.Lock()
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
		o.manifest.EntrypointChunks[bundle] = e

		if file.CssBundle != "" {
			o.manifest.Assets[bundle+".css"] = file.CssBundle[len("public"):]
		}
	}
	o.mu.Unlock()

	written := make(map[string]bool, len(r.OutputFiles))
	for _, file := range r.OutputFiles {
		written[file.Path[len(join(o.root, "public"))+1:]] = true
		if err := o.write(file.Path, file.Contents); err != nil {
			return err
		}
	}

	o.mu.Lock()
	old := o.old[desc]
	if old == nil {
		old = make(map[string]uint8, len(written))
	}
	for s, v := range old {
		if !written[s] {
			v--
		}
		if v == 0 {
			delete(o.mem, s)
			delete(o.mem, s+".gz")
			delete(old, s)
		}
		old[s] = v
	}
	for s := range written {
		old[s] = 5
	}
	o.old[desc] = old
	o.mu.Unlock()

	if err := o.writeManifest(); err != nil {
		return err
	}
	if err := o.notifyAboutBuild(desc, r); err != nil {
		return err
	}
	return nil
}

func compress(blob []byte) ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, len(blob)))
	gz, err := gzip.NewWriterLevel(buf, 6)
	if err != nil {
		return nil, errors.Tag(err, "init gzip")
	}
	if _, err = gz.Write(blob); err != nil {
		return nil, errors.Tag(err, "gzip")
	}
	if err = gz.Close(); err != nil {
		return nil, errors.Tag(err, "close gzip")
	}
	return buf.Bytes(), err
}
