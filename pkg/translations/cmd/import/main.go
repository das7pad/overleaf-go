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
	"flag"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/translations/pkg/translationsImport"
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

	license, err := os.ReadFile(path.Join(dst, "en.json.license"))
	if err != nil {
		panic(errors.Tag(err, "read license"))
	}

	filteredLocales, _, err := translationsImport.FindLocales(src, sourceDirs)
	if err != nil {
		panic(errors.Tag(err, "find locales"))
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		panic(errors.Tag(err, "iter source dir"))
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		var blob []byte
		blob, err = translationsImport.ImportLng(
			path.Join(src, entry.Name()),
			filteredLocales,
			processLocale,
		)
		if err != nil {
			panic(errors.Tag(err, "import "+entry.Name()))
		}
		p := path.Join(dst, entry.Name())
		if err = writeIfChanged(p, blob); err != nil {
			panic(errors.Tag(err, "write "+entry.Name()))
		}
		err = writeIfChanged(p+".license", license)
		if err != nil {
			panic(errors.Tag(err, "license for "+entry.Name()))
		}
	}
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
