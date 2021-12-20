// Golang port of Overleaf
// Copyright (C) 2021 Jakob Ackermann <das7pad@outlook.com>
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

package templates

import (
	"embed"
	"html/template"
)

//go:embed */*.gohtml
//go:embed *.gohtml
var _templatesRaw embed.FS

var General400 *template.Template
var General404 *template.Template
var General500 *template.Template
var GeneralUnsupportedBrowser *template.Template
var UserLogin *template.Template

func init() {
	build := func(base, content string) *template.Template {
		return template.Must(template.ParseFS(_templatesRaw, base, content))
	}
	noJS := func(content string) *template.Template {
		return build("templates/layout-no-js.gohtml", content)
	}
	marketing := func(content string) *template.Template {
		return build("templates/layout-marketing.gohtml", content)
	}
	General400 = noJS("templates/general/400.gohtml")
	General404 = marketing("templates/general/404.gohtml")
	General500 = noJS("layout-no-js.gohtml")
	GeneralUnsupportedBrowser = noJS("general/unsupported-browser.gohtml")
	UserLogin = marketing("user/login.gohtml")

	_templatesRaw = embed.FS{}
}
