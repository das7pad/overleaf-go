// Golang port of Overleaf
// Copyright (C) 2023 Jakob Ackermann <das7pad@outlook.com>
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

package frontendBuild

import (
	"archive/zip"
	"encoding/json"
	"os"
	"strings"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type yarnPNPReader struct {
	m map[string]*zip.ReadCloser
}

type yarnPNPData struct {
	PackageRegistryData [][]interface{}
}

func (r *yarnPNPReader) Load(root string, wanted map[string]bool) error {
	d := yarnPNPData{}
	blob, err := os.ReadFile(join(root, ".pnp.data.json"))
	if err != nil {
		return errors.Tag(err, "read .pnp.data.json")
	}
	if err = json.Unmarshal(blob, &d); err != nil {
		return errors.Tag(err, "deserialize .pnp.data.json")
	}
	r.m = make(map[string]*zip.ReadCloser)

	for _, l1 := range d.PackageRegistryData {
		if l1[0] == nil {
			continue
		}
		name := l1[0].(string)
		if !wanted[name] {
			continue
		}
		for _, l2 := range l1[1].([]interface{}) {
			l3 := l2.([]interface{})[1]
			meta := l3.(map[string]interface{})
			pl := meta["packageLocation"].(string)
			if !strings.HasPrefix(pl, "./.yarn/cache/") {
				continue
			}
			pl = strings.TrimSuffix(pl, "/node_modules/"+name+"/")
			z, err2 := zip.OpenReader(join(root, pl))
			if err2 != nil {
				return errors.Tag(err, pl+": open zip")
			}
			r.m[name] = z
		}
	}
	for name := range wanted {
		if _, ok := r.m[name]; !ok {
			return errors.New("missing .pnp.data.json entry: " + name)
		}
	}
	return nil
}

func (r *yarnPNPReader) Close() {
	for _, closer := range r.m {
		_ = closer.Close()
	}
}

func (r *yarnPNPReader) GetMatching(name, prefix string) map[string]*zip.File {
	out := make(map[string]*zip.File)
	inZip := "node_modules/" + name + "/"
	for _, file := range r.m[name].File {
		if strings.HasPrefix(file.Name, inZip+prefix) {
			out[file.Name[len(inZip)+len(prefix):]] = file
		}
	}
	return out
}
