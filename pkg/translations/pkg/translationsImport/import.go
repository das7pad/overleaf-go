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

package translationsImport

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

func loadLocalesInto(inLocales *map[string]string, from string) error {
	f, err := os.Open(from)
	if err != nil {
		return errors.Tag(err, "open input file "+from)
	}
	defer func() {
		_ = f.Close()
	}()
	if err = json.NewDecoder(f).Decode(inLocales); err != nil {
		return errors.Tag(err, "decode input file "+from)
	}
	return nil
}

func FindLocales(src string, sourceDirs []string) ([]string, []string, error) {
	locales := make(map[string]string)
	err := loadLocalesInto(&locales, path.Join(src, "en.json"))
	if err != nil {
		return nil, nil, errors.Tag(err, "load en locales")
	}
	lookup := make(map[byte]map[byte][][]byte)
	for s := range locales {
		b := []byte(s)
		l0, ok := lookup[b[0]]
		if !ok {
			l0 = make(map[byte][][]byte)
			lookup[b[0]] = l0
		}
		l0[b[1]] = append(l0[b[1]], b)
	}
	found := make([]string, 0, len(locales)/5)
	paths := make([]string, 0, 1000)

	for _, d := range sourceDirs {
		err = filepath.Walk(d, func(p string, f fs.FileInfo, err error) error {
			if err != nil {
				return errors.Tag(err, "walk past "+p)
			}
			switch filepath.Ext(p) {
			case ".js", ".ts", ".jsx", ".tsx":
			case ".go", ".gohtml":
			default:
				return nil
			}
			paths = append(paths, p)
			blob, err := os.ReadFile(p)
			if err != nil {
				return errors.Tag(err, "read "+p)
			}
			idx := 0
			end := len(blob) - 3
			for idx < end {
				if l0, got0 := lookup[blob[idx+1]]; got0 {
					if l1, got1 := l0[blob[idx+2]]; got1 {
						for i, v := range l1 {
							n := len(v)
							if idx+n+1 >= end {
								continue
							}
							if blob[idx] != blob[idx+n+1] {
								// matching quotes
								continue
							}
							if !bytes.Equal(blob[idx+1:idx+1+n], v) {
								continue
							}

							if len(l1) == 1 {
								delete(l0, blob[idx+2])
							} else {
								l1[i] = l1[len(l1)-1]
								l0[blob[idx+2]] = l1[:len(l1)-1]
							}

							found = append(found, string(v))
							idx += n + 1
							break
						}
					}
				}
				idx += 1
				if idx > end {
					return nil
				}
				next := bytes.IndexAny(blob[idx:], "\"'`")
				if next == -1 {
					return nil
				}
				idx += next
			}
			return nil
		})
		if err != nil {
			return nil, nil, errors.Tag(err, "walk "+src)
		}
	}
	return found, paths, nil
}

func ImportLng(in string, localeKeys []string, processLocale func(key string, v string) string) ([]byte, error) {
	inLocales := make(map[string]string)
	if err := loadLocalesInto(&inLocales, in); err != nil {
		return nil, errors.Tag(err, "target lng file")
	}
	outLocales := make(map[string]string, len(localeKeys))
	for _, key := range localeKeys {
		v, exists := inLocales[key]
		if !exists {
			// The value will be back-filled from the DefaultLang at boot-time.
			continue
		}
		outLocales[key] = processLocale(key, v)
	}

	var buf bytes.Buffer
	e := json.NewEncoder(&buf)
	e.SetIndent("", "  ")
	e.SetEscapeHTML(false)
	if err := e.Encode(outLocales); err != nil {
		return nil, errors.Tag(err, "encode locales")
	}
	return buf.Bytes(), nil
}
