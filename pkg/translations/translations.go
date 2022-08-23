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

package translations

import (
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
	out := &strings.Builder{}
	if err := l.t.Execute(out, data); err != nil {
		return "", errors.Tag(err, l.t.Name())
	}
	return template.HTML(out.String()), nil
}

//go:embed locales/*.json
var _locales embed.FS

func Load(appName string, languages []string) (Manager, error) {
	dirEntries, errListing := _locales.ReadDir("locales")
	if errListing != nil {
		return nil, errors.Tag(errListing, "cannot list locales")
	}
	byLanguage := make(map[string]map[string]renderer, len(dirEntries))
	for _, dirEntry := range dirEntries {
		language := strings.TrimSuffix(dirEntry.Name(), ".json")
		skip := true
		for _, s := range languages {
			if language == s {
				skip = false
				break
			}
		}
		if skip {
			continue
		}

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

	for _, s := range languages {
		if _, ok := byLanguage[s]; !ok {
			return nil, &errors.ValidationError{
				Msg: fmt.Sprintf("missing locales for language=%q", s),
			}
		}
	}

	_locales = embed.FS{}
	return &manager{localesByLanguage: byLanguage}, nil
}

func parseLocales(raw map[string]string, appName string) (map[string]renderer, error) {
	type minSettings struct {
		AppName string
	}
	type minData struct {
		Settings minSettings
	}
	data := &minData{Settings: minSettings{AppName: appName}}

	d := make(map[string]renderer, len(raw))
	buffer := &strings.Builder{}
	for key, s := range raw {
		t, err := template.New(key).Parse(s)
		if err != nil {
			return nil, errors.Tag(err, "cannot parse locale: "+key)
		}
		buffer.Reset()
		// Escape the template and identify static locales.
		if err = t.Execute(buffer, data); err == nil {
			d[key] = staticLocale(buffer.String())
		} else {
			d[key] = &templateLocale{t: t}
		}
	}
	return d, nil
}
