// Golang port of Overleaf
// Copyright (C) 2021-2022 Jakob Ackermann <das7pad@outlook.com>
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
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/constants"
)

type CacheBaseDir string

func (d CacheBaseDir) NamespacedCacheDir(namespace Namespace) NamespacedCacheDir {
	return NamespacedCacheDir(string(d) + "/" + string(namespace))
}

func (d CacheBaseDir) ProjectCacheDir(projectId sharedTypes.UUID) ProjectCacheDir {
	return ProjectCacheDir(string(d) + "/" + projectId.String())
}

type NamespacedCacheDir string

func (d NamespacedCacheDir) Join(name sharedTypes.PathName) string {
	return string(d) + "/" + string(name)
}

type ProjectCacheDir string

func (d ProjectCacheDir) Join(name sharedTypes.PathName) string {
	return string(d) + "/" + string(name)
}

type CompileDirBase string

func (d CompileDirBase) CompileDir(namespace Namespace) CompileDir {
	return CompileDir(string(d) + "/" + string(namespace))
}

type CompileDir string

func (d CompileDir) Join(name sharedTypes.DirEntry) string {
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

func (d CompileOutputDir) Join(name sharedTypes.PathName) string {
	return string(d) + "/" + string(name)
}

func (d CompileOutputDir) JoinDir(name sharedTypes.DirName) string {
	return string(d) + "/" + string(name)
}
