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

package sharedTypes

import (
	"strings"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type FileType string

type Filename string

func (f Filename) Validate() error {
	if f == "." || f == ".." {
		return &errors.ValidationError{Msg: "filename cannot be '.' or '..'"}
	}
	if strings.ContainsRune(string(f), '/') {
		return &errors.ValidationError{Msg: "filename cannot contain '/'"}
	}
	return nil
}

type DirEntry interface {
	Dir() DirName
	IsDir() bool
	String() string
}

type DirName string

func (d DirName) IsDir() bool {
	return true
}

func (d DirName) Dir() DirName {
	idx := strings.LastIndexByte(string(d), '/')
	if idx < 1 {
		return "."
	}
	return d[:idx]
}

func (d DirName) String() string {
	return string(d)
}

func (d DirName) Join(f Filename) PathName {
	if d == "" {
		return PathName(f)
	}
	return PathName(string(d) + "/" + string(f))
}

type PathName string

func (p PathName) Dir() DirName {
	idx := strings.LastIndexByte(string(p), '/')
	if idx < 1 {
		return "."
	}
	return DirName(p[:idx])
}

func (p PathName) IsDir() bool {
	return false
}

func (p PathName) IsStringParameter() bool {
	return true
}

func (p PathName) String() string {
	return string(p)
}

func (p PathName) Type() FileType {
	idx := strings.LastIndexByte(string(p), '.')
	if idx == -1 || idx == len(p)-1 {
		return ""
	}
	// Drop the dot.
	idx += 1
	return FileType(p[idx:])
}

func (p PathName) Validate() error {
	l := len(p)
	if l == 0 {
		return &errors.ValidationError{Msg: "empty file/path"}
	}
	if p[0] == '/' {
		return &errors.ValidationError{Msg: "file/path is absolute"}
	}
	if p == "." || p[l-1] == '/' || strings.HasSuffix(string(p), "/.") {
		return &errors.ValidationError{Msg: "file/path is dir"}
	}
	if p == ".." ||
		strings.HasPrefix(string(p), "../") ||
		strings.HasSuffix(string(p), "/..") ||
		strings.Contains(string(p), "/../") {
		return &errors.ValidationError{Msg: "file/path is jumping"}
	}
	return nil
}
