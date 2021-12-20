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
	"path"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/translations"
	"github.com/das7pad/overleaf-go/services/web/pkg/templates/internal/assets"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

//go:embed templates/*/*.gohtml
//go:embed templates/*.gohtml
var _templatesRaw embed.FS

var general400 *template.Template
var general404 *template.Template
var general500 *template.Template
var generalUnsupportedBrowser *template.Template
var userLogin *template.Template

func Load(options *types.Options) error {
	funcMap := make(template.FuncMap)
	{
		tm, err := translations.Load(options.AppName)
		if err != nil {
			return errors.Tag(err, "cannot load translations")
		}
		funcMap["getTranslationUrl"] = tm.GetTranslationUrl
		funcMap["translate"] = tm.Translate
		funcMap["translateMaybe"] = tm.TranslateMaybe
	}
	{
		am, err := assets.Load(options)
		if err != nil {
			return errors.Tag(err, "cannot load assets")
		}
		funcMap["buildCssPath"] = am.BuildCssPath
		funcMap["buildFontPath"] = am.BuildFontPath
		funcMap["buildImgPath"] = am.BuildImgPath
		funcMap["buildJsPath"] = am.BuildJsPath
		funcMap["buildMathJaxEntrypoint"] = am.BuildMathJaxEntrypoint
		funcMap["buildTPath"] = am.BuildTPath
		funcMap["getEntrypointChunks"] = am.GetEntrypointChunks
		funcMap["staticPath"] = am.StaticPath
	}

	build := func(base, content string) *template.Template {
		return template.Must(template.New(path.Base(base)).Funcs(funcMap).
			ParseFS(_templatesRaw, base, "templates/"+content),
		)
	}
	noJS := func(content string) *template.Template {
		return build("templates/layout-no-js.gohtml", content)
	}
	marketing := func(content string) *template.Template {
		return build("templates/layout-marketing.gohtml", content)
	}
	general400 = noJS("general/400.gohtml")
	general404 = marketing("general/404.gohtml")
	general500 = noJS("general/404.gohtml")
	generalUnsupportedBrowser = noJS("general/unsupported-browser.gohtml")
	userLogin = marketing("user/login.gohtml")

	_templatesRaw = embed.FS{}
	return nil
}
