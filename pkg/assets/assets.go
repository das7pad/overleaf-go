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

package assets

import (
	"bytes"
	"context"
	"encoding/json"
	"html/template"
	"io"
	"os"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/linked-url-proxy/pkg/proxyClient"
)

type Manager interface {
	BuildCSSPath(theme string) template.URL
	BuildFontPath(path string) template.URL
	BuildImgPath(path string) template.URL
	BuildMathJaxEntrypoint() template.URL
	GetBundlePath(path string) template.URL
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
	SiteURL       sharedTypes.URL
	ManifestPath  string
	WatchManifest bool
}

func Load(options Options, proxy proxyClient.Manager) (Manager, error) {
	baseURL := options.CDNURL
	if options.SiteURL.Host == options.CDNURL.Host {
		baseURL.Scheme = ""
		baseURL.Host = ""
		baseURL.OmitHost = true
	}
	m := manager{
		baseURL:          baseURL,
		assets:           map[string]template.URL{},
		entrypointChunks: map[string][]template.URL{},
	}
	if err := m.load(proxy, options.ManifestPath, options.CDNURL); err != nil {
		return nil, err
	}
	if options.WatchManifest {
		wm := watchingManager{manager: &m}
		go wm.watch(options.CDNURL)
		return &wm, nil
	}
	return &m, nil
}

type manager struct {
	assets           map[string]template.URL
	entrypointChunks map[string][]template.URL
	hints            resourceHints
	baseURL          sharedTypes.URL
}

type manifest struct {
	Assets           map[string]string   `json:"assets"`
	EntrypointChunks map[string][]string `json:"entrypointChunks"`
}

func (m *manager) load(proxy proxyClient.Manager, manifestPath string, cdnURL sharedTypes.URL) error {
	var f io.ReadCloser
	switch manifestPath {
	case "cdn":
		ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		u := cdnURL.WithPath("/manifest.json")
		body, cleanup, err := proxy.Fetch(ctx, u)
		if err != nil {
			return errors.Tag(err, "request manifest from CDN")
		}
		defer cleanup()
		f = body
	case "empty":
		f = io.NopCloser(bytes.NewReader([]byte("{}")))
	default:
		var err error
		f, err = os.Open(manifestPath)
		if err != nil {
			return errors.Tag(err, "open manifest")
		}
	}
	defer func() { _ = f.Close() }()
	return m.loadFrom(f)
}

func (m *manager) loadFrom(f io.Reader) error {
	var raw manifest
	if err := json.NewDecoder(f).Decode(&raw); err != nil {
		return errors.Tag(err, "consume manifest")
	}
	entrypointChunks := make(map[string][]template.URL, len(raw.EntrypointChunks))
	for s, urls := range raw.EntrypointChunks {
		rebased := make([]template.URL, 0, len(urls))
		for _, url := range urls {
			rebased = append(rebased, m.StaticPath(url))
		}
		entrypointChunks[s] = rebased
	}
	assets := make(map[string]template.URL)
	for s, url := range raw.Assets {
		assets[s] = m.StaticPath(url)
	}
	m.assets = assets
	m.entrypointChunks = entrypointChunks
	m.generateResourceHints()
	return nil
}

func (m *manager) RenderingStart() {}

func (m *manager) RenderingEnd() {}

func (m *manager) GetBundlePath(path string) template.URL {
	return m.assets[path]
}

func (m *manager) BuildCSSPath(theme string) template.URL {
	return m.assets["frontend/stylesheets/"+theme+"style.less"]
}

func (m *manager) BuildFontPath(path string) template.URL {
	return m.assets["frontend/fonts/"+path]
}

func (m *manager) BuildImgPath(path string) template.URL {
	return m.assets["public/img/"+path]
}

func (m *manager) BuildMathJaxEntrypoint() template.URL {
	return m.GetBundlePath("frontend/js/MathJaxBundle.js")
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
	return template.URL(m.baseURL.WithPath(path).String())
}
