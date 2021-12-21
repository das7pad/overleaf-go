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
	"io"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/translations"
	"github.com/das7pad/overleaf-go/services/web/pkg/assets"
)

type Renderer interface {
	Render() (string, error)
}

//go:embed */*.gohtml
var _fs embed.FS

var templates map[string]*template.Template

func render(p string, w io.Writer, data interface{}) error {
	return templates[p].Execute(w, data)
}

func renderOff(p string, estimate int, data interface{}) (string, error) {
	buffer := &strings.Builder{}
	buffer.Grow(estimate)
	if err := templates[p].Execute(buffer, data); err != nil {
		return "", err
	}
	return buffer.String(), nil
}

func Load(publicSettings *PublicSettings, assetsOptions *assets.Options) error {
	funcMap := make(template.FuncMap)
	{
		tm, err := translations.Load(publicSettings.AppName)
		if err != nil {
			return errors.Tag(err, "cannot load translations")
		}
		funcMap["getTranslationUrl"] = tm.GetTranslationUrl
		funcMap["translate"] = tm.Translate
	}
	{
		am, err := assets.Load(assetsOptions)
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

	// html/template does not have a mechanism for overwriting a template on a
	//  per children basis.
	// login "content" -> layout-marketing "content"
	// settings "content" -> layout-angular "content"
	// When rendering the login template, the settings "content" will be used
	//  instead of the login "content as the settings template was the last to
	//  create an override for the "content" template.
	// In the future we might be able to go back to something as simple as:
	//
	// 	all, err := template.ParseFS(_fs, "**/*.gohtml")
	//	all.ExecuteTemplate("user/login.gohtml", w, data)
	// For now, we need to work around this limitation with an extra 50 lines
	//  of boilerplate code for getting a chain of templates with copies of
	//  the respective base layouts and children on top.
	parseLayout := func(p string) (*template.Template, error) {
		l, err := template.New("").Funcs(funcMap).ParseFS(
			_fs, "layout/head.gohtml", p,
		)
		if err != nil {
			return nil, errors.Tag(err, "cannot parse "+p)
		}
		return l, nil
	}
	angularLayout, errLayout := parseLayout("layout/layout-angular.gohtml")
	if errLayout != nil {
		return errLayout
	}
	marketingLayout, errLayout := parseLayout("layout/layout-marketing.gohtml")
	if errLayout != nil {
		return errLayout
	}
	noJsLayout, errLayout := parseLayout("layout/layout-no-js.gohtml")
	if errLayout != nil {
		return errLayout
	}

	paths, errGlob := fs.Glob(_fs, "*/*.gohtml")
	if errGlob != nil {
		return errors.Tag(errGlob, "cannot glob")
	}
	templates = make(map[string]*template.Template)
	for _, p := range paths {
		if strings.HasPrefix(p, "layout/") {
			continue
		}
		blob, err := fs.ReadFile(_fs, p)
		if err != nil {
			return errors.Tag(err, "cannot read "+p)
		}
		s := string(blob)
		var c *template.Template
		if strings.Contains(s, `{{ template "layout-angular" . }}`) {
			c, err = angularLayout.Clone()
		} else if strings.Contains(s, `{{ template "layout-marketing" . }}`) {
			c, err = marketingLayout.Clone()
		} else if strings.Contains(s, `{{ template "layout-no-js" . }}`) {
			c, err = noJsLayout.Clone()
		} else {
			return errors.New("missing parent template for " + p)
		}
		if err != nil {
			return errors.Tag(err, "cannot clone layout for "+p)
		}
		templates[p], err = c.New(filepath.Base(p)).ParseFS(_fs, p)
		if err != nil {
			return errors.Tag(err, "cannot parse "+p)
		}
		// Finalize the template. With nil data, the rendering will fail fast.
		_ = templates[p].Execute(io.Discard, nil)
	}
	_fs = embed.FS{}
	return nil
}
