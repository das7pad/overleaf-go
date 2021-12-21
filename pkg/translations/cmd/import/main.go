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

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

func main() {
	in := flag.String("in", "", "input en.json")
	out := flag.String("out", "", "output en.json")
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
	inLocales := make(map[string]string)
	{
		f, err := os.Open(*in)
		if err != nil {
			panic(errors.Tag(err, "cannot open input file"))
		}
		if err = json.NewDecoder(f).Decode(&inLocales); err != nil {
			panic(errors.Tag(err, "cannot decode input file"))
		}
	}
	var inLocalesMatcher *regexp.Regexp
	{
		flat := make([]string, 0, len(inLocales))
		for key := range inLocales {
			flat = append(flat, key)
		}
		inLocalesMatcher = regexp.MustCompile(
			`"` + strings.Join(flat, `"|"`) + `"`,
		)
	}

	matches := make(map[string]bool, len(inLocales))
	for _, src := range sourceDirs {
		err := filepath.Walk(src, func(path string, f fs.FileInfo, err error) error {
			if err != nil {
				panic(errors.Tag(err, "cannot walk past "+path))
			}
			if !strings.HasSuffix(path, ".go") &&
				!strings.HasSuffix(path, ".gohtml") {
				return nil
			}
			blob, err := os.ReadFile(path)
			if err != nil {
				panic(errors.Tag(err, "cannot read "+path))
			}
			for _, m := range inLocalesMatcher.FindAllString(string(blob), -1) {
				matches[m] = true
			}
			return nil
		})
		if err != nil {
			panic(errors.Tag(err, "cannot walk "+src))
		}
	}
	outLocales := make(map[string]string, len(matches))
	for m := range matches {
		key := m[1 : len(m)-1]
		outLocales[key] = processLocale(key, inLocales[key])
	}
	{
		f, err := os.Create(*out)
		if err != nil {
			panic(errors.Tag(err, "cannot open output file"))
		}
		e := json.NewEncoder(f)
		e.SetIndent("", "  ")
		e.SetEscapeHTML(false)
		if err = e.Encode(outLocales); err != nil {
			panic(errors.Tag(err, "cannot write locales"))
		}
	}
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
	}
	if strings.Contains(v, "__") {
		panic(key + " contains __")
	}
	return v
}
