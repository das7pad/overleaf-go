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

package outputFileFinder

import (
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
	"syscall"

	"github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

type isDirMap map[types.FileName]bool
type fileStatsMap map[types.FileName]fs.DirEntry

type AllFilesAndDirs struct {
	IsDir     isDirMap
	FileStats fileStatsMap
}

// DropTree is not thread-safe.
func (a *AllFilesAndDirs) DropTree(parent types.FileName, compileDir types.CompileDir) error {
	prefix := string(parent + "/")
	dropSequence := make([]types.FileName, 0)
	for fileName := range a.IsDir {
		if fileName == parent || strings.HasPrefix(string(fileName), prefix) {
			dropSequence = append(dropSequence, fileName)
		}
	}
	// Delete deep first.
	sort.Slice(dropSequence, func(i, j int) bool {
		return dropSequence[i] > dropSequence[j]
	})
	for _, fileName := range dropSequence {
		if err := a.Delete(fileName, compileDir); err != nil {
			return err
		}
	}
	return nil
}

// Delete is not thread-safe.
func (a *AllFilesAndDirs) Delete(fileName types.FileName, compileDir types.CompileDir) error {
	p := compileDir.Join(fileName)
	var err error
	if a.IsDir[fileName] {
		err = syscall.Rmdir(p)
	} else {
		err = syscall.Unlink(p)
	}
	if err != nil {
		return fmt.Errorf("cannot delete %s: %w", p, err)
	}
	delete(a.IsDir, fileName)
	return nil
}

// EnsureIsDir is not thread-safe.
func (a *AllFilesAndDirs) EnsureIsDir(name types.FileName, compileDir types.CompileDir) error {
	if name == "." {
		// Stop at the base of the compileDir.
		return nil
	}

	if isDir, exists := a.IsDir[name]; exists {
		if isDir {
			// Happy path
			return nil
		}
		// Cleanup the clutter output file
		if err := a.Delete(name, compileDir); err != nil {
			return err
		}
		a.IsDir[name] = true
		return nil
	}

	// Make sure all the parents exists as well.
	if err := a.EnsureIsDir(name.Dir(), compileDir); err != nil {
		return err
	}
	if err := os.Mkdir(compileDir.Join(name), 0755); err != nil {
		return err
	}
	a.IsDir[name] = true
	return nil
}

// EnsureIsWritable is not thread-safe.
func (a *AllFilesAndDirs) EnsureIsWritable(name types.FileName, compileDir types.CompileDir) error {
	if isDir, exists := a.IsDir[name]; exists {
		if !isDir {
			// Happy path, overwrite doc/file
			return nil
		}
		// New doc/file placed on top of a previous output dir
		if err := a.DropTree(name, compileDir); err != nil {
			return err
		}
		a.IsDir[name] = false
		return nil
	}

	// Make sure all the parents exist and are directories.
	if err := a.EnsureIsDir(name.Dir(), compileDir); err != nil {
		return err
	}
	a.IsDir[name] = false
	return nil
}
