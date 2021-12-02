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

package fileTree

import (
	"io"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

const maxDocSize = 2 * 1024 * 1024

func IsTextFile(fileName sharedTypes.Filename, size int64, reader io.Reader) (sharedTypes.Snapshot, bool, bool, error) {
	if size > maxDocSize {
		return nil, false, false, nil
	}
	if !isTextFileFilename(fileName) {
		return nil, false, false, nil
	}
	blob := make([]byte, size)
	if _, err := io.ReadFull(reader, blob); err != nil {
		return nil, false, true, errors.Tag(err, "cannot read file")
	}
	s := sharedTypes.Snapshot(string(blob))
	if err := s.Validate(); err != nil {
		return nil, false, true, nil
	}
	return s, true, true, nil
}

func isTextFileFilename(filename sharedTypes.Filename) bool {
	if isTextFileExtension(sharedTypes.PathName(filename).Type()) {
		return true
	}
	switch filename {
	case "Dockerfile":
	case "Jenkinsfile":
	case "Makefile":
	case "latexmkrc":
	default:
		return false
	}
	return true
}

func isTextFileExtension(extension sharedTypes.FileType) bool {
	switch extension {
	case "asy":
	case "bbx":
	case "bib":
	case "bibtex":
	case "bst":
	case "cbx":
	case "clo":
	case "cls":
	case "def":
	case "dtx":
	case "editorconfig":
	case "gitignore":
	case "gv":
	case "ins":
	case "ist":
	case "latex":
	case "latexmkrc":
	case "lbx":
	case "lco":
	case "ldf":
	case "lua":
	case "m":
	case "md":
	case "mf":
	case "mtx":
	case "rmd":
	case "rtex":
	case "sty":
	case "tex":
	case "tikz":
	case "txt":
	case "yaml":
	case "yml":
	default:
		return false
	}
	return true
}
