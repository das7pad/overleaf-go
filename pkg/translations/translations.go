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

package translations

import (
	"embed"
	"encoding/json"
	"html/template"
	"net/url"
	"regexp"
	"strings"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Manager interface {
	GetTranslationUrl(lng string) template.URL
	Translate(key string, data languageGetter) (template.HTML, error)
}

type manager struct {
	localesByLanguage map[string]map[string]renderer
	siteURL           sharedTypes.URL
}

func (m *manager) GetTranslationUrl(lng string) template.URL {
	u := m.siteURL.WithQuery(url.Values{
		"setGlobalLng": {lng},
	})
	return template.URL(u.String())
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

type simpleLocale string

func (l simpleLocale) Render(interface{}) (template.HTML, error) {
	return template.HTML(l), nil
}

type templateLocale struct {
	t *template.Template
}

func (l *templateLocale) Render(data interface{}) (template.HTML, error) {
	out := &strings.Builder{}
	if err := l.t.Execute(out, data); err != nil {
		return "", errors.Tag(err, l.t.Name())
	}
	return template.HTML(out.String()), nil
}

//go:embed locales/*.json
var _locales embed.FS

func Load(appName string) (Manager, error) {
	dirEntries, errListing := _locales.ReadDir("locales")
	if errListing != nil {
		return nil, errors.Tag(errListing, "cannot list locales")
	}
	byLanguage := make(map[string]map[string]renderer, len(dirEntries))
	for _, dirEntry := range dirEntries {
		language := strings.TrimSuffix(dirEntry.Name(), ".json")
		f, errOpen := _locales.Open("locales/" + dirEntry.Name())
		if errOpen != nil {
			return nil, errors.Tag(errOpen, "cannot open: "+language)
		}
		raw := make(map[string]string)
		if err := json.NewDecoder(f).Decode(&raw); err != nil {
			return nil, errors.Tag(err, "cannot consume: "+language)
		}
		d, err := parseLocales(raw, appName)
		if err != nil {
			return nil, errors.Tag(err, "cannot parse locales: "+language)
		}
		byLanguage[language] = d
	}
	_locales = embed.FS{}
	return &manager{localesByLanguage: byLanguage}, nil
}

func parseLocales(raw map[string]string, appName string) (map[string]renderer, error) {
	appNameRegex := regexp.MustCompile("{{ .Settings.AppName }}")

	d := make(map[string]renderer, len(raw))
	for key, s := range raw {
		s = appNameRegex.ReplaceAllString(s, appName)
		if !strings.Contains(s, "{{") && !strings.Contains(s, "<") {
			d[key] = simpleLocale(template.HTMLEscapeString(s))
			continue
		}
		t, err := template.New(key).Parse(s)
		if err != nil {
			return nil, errors.Tag(err, "cannot parse locale: "+key)
		}
		d[key] = &templateLocale{t: t}
	}
	return d, nil
}
