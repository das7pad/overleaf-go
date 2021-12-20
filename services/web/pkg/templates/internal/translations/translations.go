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
	"regexp"
	"strings"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type TranslateFn func(key string, pairs ...string) template.HTML

func TranslateInto(language string) TranslateFn {
	return byLanguage[language]
}

var fields = regexp.MustCompile("__.+?__")

func translateFnFor(d map[string]string) TranslateFn {
	return func(key string, pairs ...string) template.HTML {
		v, exists := d[key]
		if !exists {
			return template.HTML(key)
		}
		if len(pairs) == 0 || !strings.ContainsRune(v, '_') {
			return template.HTML(v)
		}
		out := &strings.Builder{}
		out.Grow(len(v))
		consumed := 0
		for _, bounds := range fields.FindAllStringIndex(v, -1) {
			start, end := bounds[0], bounds[1]
			if start > consumed {
				out.WriteString(v[consumed:start])
			}
			search := v[start+2 : end-2]
			for i := 0; i < len(pairs)/2; i++ {
				if pairs[2*i] == search {
					template.HTMLEscape(out, []byte(pairs[2*i+1]))
					consumed = end
					break
				}
			}
		}
		if consumed < len(v) {
			out.WriteString(v[consumed:])
		}
		return template.HTML(out.String())
	}
}

var byLanguage map[string]TranslateFn

//go:embed locales/*.json
var locales embed.FS

func init() {
	dirEntries, errListing := locales.ReadDir("locales")
	if errListing != nil {
		panic(errListing)
	}
	byLanguage = make(map[string]TranslateFn)
	for _, dirEntry := range dirEntries {
		language := strings.TrimSuffix(dirEntry.Name(), ".json")
		f, err := locales.Open("locales/" + dirEntry.Name())
		if err != nil {
			panic(errors.Tag(err, "cannot open: "+dirEntry.Name()))
		}
		d := make(map[string]string)
		if err = json.NewDecoder(f).Decode(&d); err != nil {
			panic(errors.Tag(err, "cannot consume: "+dirEntry.Name()))
		}
		byLanguage[language] = translateFnFor(d)
	}
	locales = embed.FS{}
}
