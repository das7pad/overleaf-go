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
	"unicode"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

const maxDocSize = 2 * 1024 * 1024

func isTextFile(request *types.UploadFileRequest) (sharedTypes.Snapshot, bool, error) {
	if request.Size > maxDocSize {
		return nil, false, nil
	}
	if !isTextFileFilename(request.FileName) {
		return nil, false, nil
	}
	blob := make([]byte, request.Size)
	_, err := io.ReadFull(request.File, blob)
	if err != nil {
		return nil, false, errors.Tag(err, "cannot read file")
	}
	s := sharedTypes.Snapshot(string(blob))
	for _, r := range s {
		if r == '\x00' || unicode.Is(unicode.Cs, r) {
			return nil, false, nil
		}
	}
	return s, true, nil
}

func isTextFileFilename(filename sharedTypes.Filename) bool {
	if filename == "latexmkrc" || filename == ".latexmkrc" {
		return true
	}
	extension := sharedTypes.PathName(filename).Type()
	switch extension {
	case "tex":
	case "latex":
	case "sty":
	case "cls":
	case "bst":
	case "bib":
	case "bibtex":
	case "txt":
	case "tikz":
	case "mtx":
	case "rtex":
	case "md":
	case "asy":
	case "latexmkrc":
	case "lbx":
	case "bbx":
	case "cbx":
	case "m":
	case "lco":
	case "dtx":
	case "ins":
	case "ist":
	case "def":
	case "clo":
	case "ldf":
	case "rmd":
	case "lua":
	case "gv":
	case "mf":
	case "yml":
	case "yaml":
	default:
		return false
	}
	return true
}
