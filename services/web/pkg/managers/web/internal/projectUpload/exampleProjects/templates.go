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

package exampleProjects

import (
	"embed"
	"io/fs"
	"strings"
	"text/template"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web/internal/fileTree"
)

//go:embed templates/**/*.gohtml
//go:embed templates/**/*.jpg
var _templatesRaw embed.FS

func init() {
	projectDirs, errListProjects := _templatesRaw.ReadDir("templates")
	if errListProjects != nil {
		panic(errListProjects)
	}
	for _, projectDir := range projectDirs {
		name := projectDir.Name()
		projectPath := "templates/" + name

		binaryFiles := make([]*binaryFile, 0)
		textFiles := make([]*textFile, 0)
		err := fs.WalkDir(
			_templatesRaw,
			projectPath,
			func(pathInFS string, d fs.DirEntry, errWalk error) error {
				if errWalk != nil {
					panic(errors.Tag(errWalk, pathInFS))
				}
				if d.IsDir() {
					return nil
				}
				blob, errRead := _templatesRaw.ReadFile(pathInFS)
				if errRead != nil {
					panic(errors.Tag(errRead, "read: "+pathInFS))
				}

				path := sharedTypes.PathName(
					strings.TrimPrefix(pathInFS, projectPath+"/"),
				)
				if path.Type() == "gohtml" {
					path = path[:len(path)-len(".gohtml")]
					t, err := template.ParseFS(_templatesRaw, pathInFS)
					if err != nil {
						panic(errors.Tag(err, "parse: "+pathInFS))
					}
					textFiles = append(textFiles, &textFile{
						path:     path,
						template: t,
						size:     int64(len(blob)),
					})
				} else {
					f := &binaryFile{
						path: path,
						blob: blob,
						size: int64(len(blob)),
					}
					hash, err := fileTree.HashFile(f.reader(), f.size)
					if errRead != nil {
						panic(errors.Tag(err, "hash: "+pathInFS))
					}
					f.hash = hash
					binaryFiles = append(binaryFiles, f)
				}
				return nil
			},
		)
		if err != nil {
			panic(errors.Tag(err, "walk: "+name))
		}
		projects[name] = &projectContent{
			textFiles:   textFiles,
			binaryFiles: binaryFiles,
		}
	}
	_templatesRaw = embed.FS{}
}
