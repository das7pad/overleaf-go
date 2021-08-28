// Golang port of the Overleaf clsi service
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

package types

import (
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/constants"
)

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

type FileName string

func (f FileName) Dir() DirName {
	idx := strings.LastIndexByte(string(f), '/')
	if idx < 1 {
		return "."
	}
	return DirName(f[:idx])
}

func (f FileName) IsDir() bool {
	return false
}

func (f FileName) IsStringParameter() bool {
	return true
}

func (f FileName) String() string {
	return string(f)
}

func (f FileName) Type() FileType {
	idx := strings.LastIndexByte(string(f), '.')
	if idx == -1 || idx == len(f)-1 {
		return ""
	}
	// Drop the dot.
	idx += 1
	return FileType(f[idx:])
}

func (f FileName) Validate() error {
	l := len(f)
	if l == 0 {
		return &errors.ValidationError{Msg: "empty file/path"}
	}
	if f[0] == '/' {
		return &errors.ValidationError{Msg: "file/path is absolute"}
	}
	if f == "." || f[l-1] == '/' || strings.HasSuffix(string(f), "/.") {
		return &errors.ValidationError{Msg: "file/path is dir"}
	}
	if f == ".." ||
		strings.HasPrefix(string(f), "../") ||
		strings.HasSuffix(string(f), "/..") ||
		strings.Contains(string(f), "/../") {
		return &errors.ValidationError{Msg: "file/path is jumping"}
	}
	return nil
}

type CacheBaseDir string

func (d CacheBaseDir) NamespacedCacheDir(namespace Namespace) NamespacedCacheDir {
	return NamespacedCacheDir(string(d) + "/" + string(namespace))
}

func (d CacheBaseDir) ProjectCacheDir(projectId primitive.ObjectID) ProjectCacheDir {
	return ProjectCacheDir(string(d) + "/" + projectId.Hex())
}

type NamespacedCacheDir string

func (d NamespacedCacheDir) Join(name FileName) string {
	return string(d) + "/" + string(name)
}

type ProjectCacheDir string

func (d ProjectCacheDir) Join(name FileName) string {
	return string(d) + "/" + string(name)
}

type CompileDirBase string

func (d CompileDirBase) CompileDir(namespace Namespace) CompileDir {
	return CompileDir(string(d) + "/" + string(namespace))
}

type CompileDir string

func (d CompileDir) Join(name DirEntry) string {
	return string(d) + "/" + name.String()
}

type OutputBaseDir string

func (d OutputBaseDir) OutputDir(namespace Namespace) OutputDir {
	return OutputDir(string(d) + "/" + string(namespace))
}

type OutputDir string

func (d OutputDir) CompileOutput() string {
	return string(d) + "/" + constants.CompileOutputLabel
}

func (d OutputDir) CompileOutputDir(id BuildId) CompileOutputDir {
	return CompileOutputDir(d.CompileOutput() + "/" + string(id))
}

type CompileOutputDir string

func (d CompileOutputDir) Join(name FileName) string {
	return string(d) + "/" + string(name)
}

func (d CompileOutputDir) JoinDir(name DirName) string {
	return string(d) + "/" + string(name)
}
