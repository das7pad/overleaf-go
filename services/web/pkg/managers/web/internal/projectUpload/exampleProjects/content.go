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
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type textFile struct {
	path     sharedTypes.PathName
	template *template.Template
	size     int64
}

type renderedTextFile struct {
	path sharedTypes.PathName
	blob []byte
	size int64
}

func (f *renderedTextFile) Open() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(f.blob)), nil
}

func (f *renderedTextFile) Size() int64 {
	return f.size
}

func (f *renderedTextFile) Path() sharedTypes.PathName {
	return f.path
}

func (f *renderedTextFile) PreComputedHash() sharedTypes.Hash {
	return ""
}

type binaryFile struct {
	path sharedTypes.PathName
	blob []byte
	hash sharedTypes.Hash
	size int64
}

func (f *binaryFile) Open() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(f.blob)), nil
}

func (f *binaryFile) Size() int64 {
	return f.size
}

func (f *binaryFile) Path() sharedTypes.PathName {
	return f.path
}

func (f *binaryFile) PreComputedHash() sharedTypes.Hash {
	return f.hash
}

type projectContent struct {
	textFiles   []*textFile
	binaryFiles []*binaryFile
}

func (p *projectContent) Render(data *ViewData) ([]types.CreateProjectFile, error) {
	nDocs := len(p.textFiles)
	files := make([]types.CreateProjectFile, nDocs+len(p.binaryFiles))
	for i, doc := range p.textFiles {
		b := bytes.NewBuffer(make([]byte, 0, doc.size+2048))
		if err := doc.template.Execute(b, data); err != nil {
			return nil, errors.Tag(err, doc.path.String())
		}
		files[i] = &renderedTextFile{
			path: doc.path,
			blob: b.Bytes(),
			size: int64(b.Len()),
		}
	}
	for i, file := range p.binaryFiles {
		files[nDocs+i] = file
	}
	return files, nil
}

var projects = make(map[string]*projectContent, 0)
