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

package translations

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/url"
	"strings"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Manager interface {
	GetTranslationURL(lng string) (template.URL, error)
	Translate(key string, data languageGetter) (template.HTML, error)
}

type manager struct {
	localesByLanguage map[string]map[string]renderer
	siteURL           sharedTypes.URL
}

func (m *manager) GetTranslationURL(lng string) (template.URL, error) {
	if _, ok := m.localesByLanguage[lng]; !ok {
		return "", &errors.InvalidStateError{Msg: "unknown language specified"}
	}
	u := m.siteURL.WithPath("/switch-language").WithQuery(url.Values{
		"lng": {lng},
	})
	return template.URL(u.String()), nil
}

func (m *manager) Translate(key string, data languageGetter) (template.HTML, error) {
	return m.localesByLanguage[data.CurrentLngCode()][key].Render(data)
}

type languageGetter interface {
	CurrentLngCode() string
}

type renderer interface {
	Render(data interface{}) (template.HTML, error)
}

type staticLocale string

func (l staticLocale) Render(interface{}) (template.HTML, error) {
	return template.HTML(l), nil
}

type templateLocale struct {
	t *template.Template
}

func (l *templateLocale) Render(data interface{}) (template.HTML, error) {
	out := strings.Builder{}
	if err := l.t.Execute(&out, data); err != nil {
		return "", errors.Tag(err, l.t.Name())
	}
	return template.HTML(out.String()), nil
}

//go:embed locales/*.json
var _locales embed.FS

var validLanguages Languages

func init() {
	dirEntries, errListing := _locales.ReadDir("locales")
	if errListing != nil {
		panic(errors.Tag(errListing, "list locales"))
	}
	for _, dirEntry := range dirEntries {
		language := strings.TrimSuffix(dirEntry.Name(), ".json")
		validLanguages = append(validLanguages, language)
	}
}

type Languages []string

func (l Languages) Validate() error {
	if len(l) == 0 {
		return &errors.ValidationError{Msg: "no languages provided"}
	}
	for _, s := range l {
		if !validLanguages.Has(s) {
			return &errors.ValidationError{
				Msg: fmt.Sprintf("%q is not a valid language", s),
			}
		}
	}
	return nil
}

func (l Languages) Has(other string) bool {
	for _, s := range l {
		if s == other {
			return true
		}
	}
	return false
}

func Load(appName string, defaultLang string, languages Languages) (Manager, error) {
	if err := (Languages{defaultLang}).Validate(); err != nil {
		return nil, err
	}
	if err := languages.Validate(); err != nil {
		return nil, err
	}
	byLanguage := make(map[string]map[string]renderer, len(languages))
	for _, language := range languages {
		d, err := load(language, appName)
		if err != nil {
			return nil, err
		}
		byLanguage[language] = d
	}
	fallback := byLanguage[defaultLang]
	if len(fallback) == 0 {
		d, err := load(defaultLang, appName)
		if err != nil {
			return nil, err
		}
		fallback = d
	}
	if defaultLang != "en" {
		d, err := load("en", appName)
		if err != nil {
			return nil, err
		}
		fallback = merge(fallback, d)
	}
	for language, d := range byLanguage {
		byLanguage[language] = merge(d, fallback)
	}
	_locales = embed.FS{}
	return &manager{localesByLanguage: byLanguage}, nil
}

func merge(dst, src map[string]renderer) map[string]renderer {
	for key, r := range src {
		if _, exists := dst[key]; !exists {
			dst[key] = r
		}
	}
	return dst
}

func load(language string, appName string) (map[string]renderer, error) {
	f, errOpen := _locales.Open("locales/" + language + ".json")
	if errOpen != nil {
		return nil, errors.Tag(errOpen, "open "+language)
	}
	defer func() { _ = f.Close() }()
	raw := make(map[string]string)
	if err := json.NewDecoder(f).Decode(&raw); err != nil {
		return nil, errors.Tag(err, "consume "+language)
	}
	d, err := parseLocales(raw, appName)
	if err != nil {
		return nil, errors.Tag(err, "parse "+language)
	}
	return d, err
}

func parseLocales(raw map[string]string, appName string) (map[string]renderer, error) {
	type minSettings struct {
		AppName string
	}
	type minData struct {
		Settings minSettings
	}
	data := minData{Settings: minSettings{AppName: appName}}

	d := make(map[string]renderer, len(raw))
	buffer := bytes.Buffer{}
	for key, s := range raw {
		t, err := template.New(key).Parse(s)
		if err != nil {
			return nil, errors.Tag(err, "parse locale: "+key)
		}
		buffer.Reset()
		// Escape the template and identify static locales.
		if err = t.Execute(&buffer, data); err == nil {
			d[key] = staticLocale(buffer.String())
		} else {
			d[key] = &templateLocale{t: t}
		}
	}
	return d, nil
}
