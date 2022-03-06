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
	"bytes"
	"io"
	"text/template"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type textFile struct {
	path     sharedTypes.PathName
	template *template.Template
	size     int64
}

type binaryFile struct {
	path sharedTypes.PathName
	blob []byte
	hash sharedTypes.Hash
	size int64
}

func (f *binaryFile) reader() io.Reader {
	return bytes.NewReader(f.blob)
}

type projectContent struct {
	textFiles   []*textFile
	binaryFiles []*binaryFile
}

func (p *projectContent) Render(data *ViewData) ([]Doc, []File, error) {
	docs := make([]Doc, len(p.textFiles))
	for i, doc := range p.textFiles {
		b := bytes.NewBuffer(make([]byte, 0, doc.size))
		if err := doc.template.Execute(b, data); err != nil {
			return nil, nil, errors.Tag(err, doc.path.String())
		}
		docs[i].Path = doc.path
		docs[i].Snapshot = sharedTypes.Snapshot(b.String())
	}
	files := make([]File, len(p.binaryFiles))
	for i, file := range p.binaryFiles {
		files[i].Hash = file.hash
		files[i].Path = file.path
		files[i].Size = file.size
		files[i].Reader = file.reader()
	}
	return docs, files, nil
}

var projects = make(map[string]*projectContent, 0)
