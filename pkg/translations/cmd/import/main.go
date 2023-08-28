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

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

func main() {
	in := flag.String("in", "", "source locales/")
	out := flag.String("out", "", "destination locales/")
	flag.Parse()
	if *in == "" || *out == "" {
		flag.Usage()
		os.Exit(101)
	}
	sourceDirs := flag.Args()
	if len(sourceDirs) == 0 {
		fmt.Println("ERR: must specify at least one source directory")
		os.Exit(101)
	}
	src := *in
	dst := *out

	var license []byte
	if l, err := os.ReadFile(path.Join(dst, "en.json.license")); err != nil {
		panic(errors.Tag(err, "read license"))
	} else {
		license = l
	}

	filteredLocales := findLocales(src, sourceDirs)

	entries, errIterSourceDir := os.ReadDir(src)
	if errIterSourceDir != nil {
		panic(errors.Tag(errIterSourceDir, "iter source dir"))
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		err := importLng(
			path.Join(dst, entry.Name()),
			path.Join(src, entry.Name()),
			filteredLocales,
		)
		if err != nil {
			panic(errors.Tag(err, entry.Name()))
		}
		err = writeIfChanged(path.Join(dst, entry.Name()+".license"), license)
		if err != nil {
			panic(errors.Tag(err, "license for "+entry.Name()))
		}
	}
}

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

func findLocales(src string, sourceDirs []string) []string {
	locales := make(map[string]string)
	err := loadLocalesInto(&locales, path.Join(src, "en.json"))
	if err != nil {
		panic(errors.Tag(err, "load en locales"))
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

	for _, d := range sourceDirs {
		err = filepath.Walk(d, func(path string, f fs.FileInfo, err error) error {
			if err != nil {
				return errors.Tag(err, "walk past "+path)
			}
			switch filepath.Ext(path) {
			case ".go", ".gohtml":
			default:
				return nil
			}
			blob, err := os.ReadFile(path)
			if err != nil {
				return errors.Tag(err, "read "+path)
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
			panic(errors.Tag(err, "walk "+src))
		}
	}
	return found
}

func writeLocales(out string, locales map[string]string) error {
	var buf bytes.Buffer
	e := json.NewEncoder(&buf)
	e.SetIndent("", "  ")
	e.SetEscapeHTML(false)
	if err := e.Encode(locales); err != nil {
		return errors.Tag(err, "write locales")
	}
	return writeIfChanged(out, buf.Bytes())
}

func writeIfChanged(dst string, blob []byte) error {
	if old, err := os.ReadFile(dst); err == nil && bytes.Equal(blob, old) {
		return nil
	}
	if err := os.WriteFile(dst, blob, 0o644); err != nil {
		return errors.Tag(err, "write "+dst)
	}
	return nil
}

func importLng(out, in string, localeKeys []string) error {
	inLocales := make(map[string]string)
	if err := loadLocalesInto(&inLocales, in); err != nil {
		return errors.Tag(err, "target lng file")
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
	if err := writeLocales(out, outLocales); err != nil {
		return errors.Tag(err, "write out")
	}
	return nil
}

func processLocale(key, v string) string {
	v = strings.ReplaceAll(v, "__appName__", "{{ .Settings.AppName }}")
	switch key {
	case "user_wants_you_to_see_project":
		v = strings.ReplaceAll(v, "__username__", "{{ .SharedProjectData.UserName }}")
		v = strings.ReplaceAll(v, "__projectname__", "<em>{{ .SharedProjectData.ProjectName }}</em>")
	case "notification_project_invite":
		// NOTE: This is a virtual key used for displaying the CTA notification
		//        in the project dashboard. Other locales take over the actual
		//        display.
		v = "-"
	case "account_with_email_exists":
		v = strings.ReplaceAll(v, "the email <b>__email__</b>", "the provided email")
	case "reconnecting_in_x_secs":
		v = strings.ReplaceAll(v, "__seconds__", "{{ `{{ connection.reconnection_countdown }}` }}")
	case "saving_notification_with_seconds":
		//goland:noinspection SpellCheckingInspection
		v = strings.ReplaceAll(v, "__docname__", "{{ `{{ state.doc.name }}` }}")
		v = strings.ReplaceAll(v, "__seconds__", "{{ `{{ state.unsavedSeconds }}` }}")
	case "file_has_been_deleted", "file_restored":
		v = strings.ReplaceAll(v, "__filename__", "{{ `{{ history.diff.doc.name }}` }}")
	case "sure_you_want_to_restore_before":
		v = strings.ReplaceAll(v, "__filename__", "{{ `{{ diff.doc.name }}` }}")
		v = strings.ReplaceAll(v, "__date__", "{{ `{{ diff.start_ts | formatDate }}` }}")
		v = strings.ReplaceAll(v, "<0>", "<strong>")
		v = strings.ReplaceAll(v, "</0>", "</strong>")
	}
	if strings.Contains(v, "__") || strings.Contains(v, "<0>") {
		panic(key + " needs processing: " + v)
	}
	return v
}
