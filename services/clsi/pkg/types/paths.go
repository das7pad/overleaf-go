// Golang port of Overleaf
// Copyright (C) 2021-2024 Jakob Ackermann <das7pad@outlook.com>
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

func (d CacheBaseDir) StateFile(namespace Namespace) string {
	return string(d) + "/state/" + string(namespace) + constants.ProjectSyncStateFilename
}

func (d CacheBaseDir) ProjectCacheDir(namespace Namespace) ProjectCacheDir {
	return ProjectCacheDir(string(d) + "/" + string(namespace[:36]))
}

type ProjectCacheDir string

func (d ProjectCacheDir) Join(name sharedTypes.PathName) string {
	return string(d) + "/" + string(name)
}

type CompileBaseDir string

func (d CompileBaseDir) CompileDir(namespace Namespace) CompileDir {
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

func (d OutputDir) Tracker() string {
	return string(d) + "/" + constants.PDFCachingTrackerFilename
}

func (d OutputDir) CompileOutput() string {
	return string(d) + "/" + constants.CompileOutputLabel
}

func (d OutputDir) CompileOutputDir(id BuildId) CompileOutputDir {
	return CompileOutputDir(d.CompileOutput() + "/" + string(id))
}

func (d OutputDir) ContentDirBase() string {
	return string(d) + "/" + constants.ContentLabel
}

func (d OutputDir) ContentDir(id BuildId) ContentDir {
	return ContentDir(string(d) + "/" + constants.ContentLabel + "/" + string(id))
}

type ContentDir string

func (d ContentDir) Join(hash sharedTypes.Hash) string {
	return string(d) + "/" + string(hash)
}

type CompileOutputDir string

func (d CompileOutputDir) Join(name sharedTypes.PathName) string {
	return string(d) + "/" + string(name)
}

func (d CompileOutputDir) JoinDir(name sharedTypes.DirName) string {
	return string(d) + "/" + string(name)
}

type Paths struct {
	CacheBaseDir   CacheBaseDir   `json:"cache_base_dir"`
	CompileBaseDir CompileBaseDir `json:"compile_base_dir"`
	OutputBaseDir  OutputBaseDir  `json:"output_base_dir"`
}
