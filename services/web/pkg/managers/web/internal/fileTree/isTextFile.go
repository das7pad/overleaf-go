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

package fileTree

import (
	"io"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

func IsTextFile(fileName sharedTypes.Filename, size int64, reader io.Reader) (sharedTypes.Snapshot, bool, bool, error) {
	if size > sharedTypes.MaxDocSizeBytes {
		return nil, false, false, nil
	}
	if !isTextFileFilename(fileName) {
		return nil, false, false, nil
	}
	blob := make([]byte, size)
	if _, err := io.ReadFull(reader, blob); err != nil {
		return nil, false, true, errors.Tag(err, "read file")
	}
	s := sharedTypes.Snapshot(string(blob))
	if editable := s.Validate() == nil; !editable {
		return nil, false, true, nil
	}
	return s, true, true, nil
}

func isTextFileFilename(filename sharedTypes.Filename) bool {
	if sharedTypes.PathName(filename).Type().ValidForDoc() {
		return true
	}
	//goland:noinspection SpellCheckingInspection
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
