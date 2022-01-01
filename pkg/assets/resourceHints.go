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
	"strings"
)

type resourceHints struct {
	Default       string
	EditorDefault string
	EditorLight   string
}

type resource struct {
	uri         string
	as          string
	crossOrigin bool
}

type resources []resource

func (s resources) flatten() string {
	b := &strings.Builder{}
	for i, r := range s {
		if i != 0 {
			b.WriteRune(',')
		}
		b.WriteRune('<')
		b.WriteString(r.uri)
		b.WriteString(">;rel=preload;as=")
		b.WriteString(r.as)
		if r.crossOrigin {
			//goland:noinspection SpellCheckingInspection
			b.WriteString(";crossorigin")
		}
	}
	return b.String()
}

func (m *manager) generateResourceHints() {
	font := func(path string) resource {
		return resource{
			uri:         string(m.BuildFontPath(path)),
			as:          "font",
			crossOrigin: true,
		}
	}
	style := func(theme string) resource {
		return resource{
			uri: string(m.BuildCssPath(theme)),
			as:  "style",
		}
	}
	image := func(path string) resource {
		return resource{
			uri: string(m.BuildImgPath(path)),
			as:  "image",
		}
	}

	editor := func(theme string) resources {
		return resources{
			style(theme),
			font("merriweather-v21-latin-regular.woff2"),
			image("ol-brand/overleaf-o.svg"),
			image("ol-brand/overleaf-o-grey.svg"),
		}
	}

	m.hints = resourceHints{
		Default: resources{
			style(""),
			font("merriweather-v21-latin-regular.woff2"),
		}.flatten(),
		EditorDefault: editor("").flatten(),
		EditorLight:   editor("light-").flatten(),
	}
}
