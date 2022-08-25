// Golang port of Overleaf
// Copyright (C) 2021-2022 Jakob Ackermann <das7pad@outlook.com>
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

package assets

import (
	"encoding/json"
	"html/template"
	"os"
	"strings"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Manager interface {
	BuildCssPath(theme string) template.URL
	BuildFontPath(path string) template.URL
	BuildImgPath(path string) template.URL
	BuildJsPath(path string) template.URL
	BuildMathJaxEntrypoint() template.URL
	BuildTPath(lng string) template.URL
	GetEntrypointChunks(path string) []template.URL
	StaticPath(path string) template.URL
	ResourceHintsManager
}

type ResourceHintsManager interface {
	RenderingStart()
	RenderingEnd()
	ResourceHintsDefault() string
	ResourceHintsEditorDefault() string
	ResourceHintsEditorLight() string
}

type Options struct {
	CDNURL        sharedTypes.URL
	ManifestPath  string
	WatchManifest bool
}

func Load(options Options) (Manager, error) {
	base := template.URL(strings.TrimSuffix(options.CDNURL.String(), "/"))
	m := &manager{
		manifestPath:     options.ManifestPath,
		base:             base,
		assets:           map[string]template.URL{},
		entrypointChunks: map[string][]template.URL{},
	}
	if err := m.load(); err != nil {
		return nil, err
	}
	if options.WatchManifest {
		wm := &watchingManager{manager: m}
		go wm.watch()
		return wm, nil
	}
	return m, nil
}

type manager struct {
	manifestPath     string
	base             template.URL
	assets           map[string]template.URL
	entrypointChunks map[string][]template.URL
	hints            resourceHints
}

type manifest struct {
	Assets           map[string]template.URL   `json:"assets"`
	EntrypointChunks map[string][]template.URL `json:"entrypointChunks"`
}

func (m *manager) load() error {
	f, err := os.Open(m.manifestPath)
	if err != nil {
		return errors.Tag(err, "cannot open manifest")
	}
	var raw manifest
	if err = json.NewDecoder(f).Decode(&raw); err != nil {
		return errors.Tag(err, "cannot consume manifest")
	}
	entrypointChunks := make(map[string][]template.URL, len(raw.EntrypointChunks))
	for s, urls := range raw.EntrypointChunks {
		rebased := make([]template.URL, 0, len(urls))
		for _, url := range urls {
			rebased = append(rebased, m.base+url)
		}
		entrypointChunks[s] = rebased
	}
	assets := make(map[string]template.URL)
	for s, url := range raw.Assets {
		assets[s] = m.base + url
	}
	m.assets = assets
	m.entrypointChunks = entrypointChunks
	m.generateResourceHints()
	return nil
}

func (m *manager) RenderingStart() {}

func (m *manager) RenderingEnd() {}

func (m *manager) BuildCssPath(theme string) template.URL {
	return m.assets["frontend/stylesheets/"+theme+"style.less"]
}

func (m *manager) BuildFontPath(path string) template.URL {
	return m.assets["frontend/fonts/"+path]
}

func (m *manager) BuildImgPath(path string) template.URL {
	return m.assets["public/img/"+path]
}

func (m *manager) BuildJsPath(path string) template.URL {
	return m.assets["frontend/js/"+path]
}

func (m *manager) BuildMathJaxEntrypoint() template.URL {
	return m.BuildJsPath("MathJaxBundle.js")
}

func (m *manager) BuildTPath(lng string) template.URL {
	return m.assets["generated/lng/"+lng+".js"]
}

func (m *manager) GetEntrypointChunks(path string) []template.URL {
	return m.entrypointChunks[path]
}

func (m *manager) ResourceHintsDefault() string {
	return m.hints.Default
}

func (m *manager) ResourceHintsEditorDefault() string {
	return m.hints.EditorDefault
}

func (m *manager) ResourceHintsEditorLight() string {
	return m.hints.EditorLight
}

func (m *manager) StaticPath(path string) template.URL {
	return m.base + template.URL(path)
}
