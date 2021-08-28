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
	"io/fs"
	"os"
	"sort"
	"strings"
	"syscall"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

type dirEntriesMap map[string]types.DirEntry
type fileStatsMap map[types.FileName]fs.DirEntry

type AllFilesAndDirs struct {
	DirEntries dirEntriesMap
	FileStats  fileStatsMap
}

// DropTree is not thread-safe.
func (a *AllFilesAndDirs) DropTree(parent types.DirEntry, compileDir types.CompileDir) error {
	exactMatch := parent.String()
	prefix := exactMatch + "/"
	dropSequence := make([]string, 0)
	for fileName := range a.DirEntries {
		if fileName == exactMatch || strings.HasPrefix(fileName, prefix) {
			dropSequence = append(dropSequence, fileName)
		}
	}
	// Delete deep first.
	sort.Slice(dropSequence, func(i, j int) bool {
		return dropSequence[i] > dropSequence[j]
	})
	for _, fileName := range dropSequence {
		if err := a.Delete(a.DirEntries[fileName], compileDir); err != nil {
			return err
		}
	}
	return nil
}

// Delete is not thread-safe.
func (a *AllFilesAndDirs) Delete(entry types.DirEntry, compileDir types.CompileDir) error {
	p := compileDir.Join(entry)
	if entry.IsDir() {
		if err := syscall.Rmdir(p); err != nil {
			return errors.Tag(err, "cannot delete directory "+p)
		}
	} else {
		if err := syscall.Unlink(p); err != nil {
			return errors.Tag(err, "cannot delete file "+p)
		}
	}
	delete(a.DirEntries, entry.String())
	return nil
}

// EnsureIsDir is not thread-safe.
func (a *AllFilesAndDirs) EnsureIsDir(name types.DirName, compileDir types.CompileDir) error {
	s := name.String()

	// Step 0: Bail out at the base of the compileDir.
	if s == "." {
		return nil
	}

	// Step 1: action on what already exists in the in-memory view of the fs.
	if entry, exists := a.DirEntries[s]; exists && entry.IsDir() {
		// Happy path, already exists as directory.
		return nil
	} else if exists {
		// Entry is a file instead of a directory, delete it first.
		// Case A: The last compile has replaced the directory with a file.
		// Case B: The user has restructured the tree since the last compile.
		if err := a.Delete(entry, compileDir); err != nil {
			return err
		}
	} else {
		// New directory, create parent directories first.
		if err := a.EnsureIsDir(name.Dir(), compileDir); err != nil {
			return err
		}
	}

	// Step 2: create the directory.
	p := compileDir.Join(name)
	if err := os.Mkdir(p, 0755); err != nil {
		return errors.Tag(err, "cannot create directory "+p)
	}

	// Step 3: persist new state in the in-memory view of the fs.
	a.DirEntries[s] = name
	return nil
}

// EnsureIsWritable is not thread-safe.
func (a *AllFilesAndDirs) EnsureIsWritable(name types.FileName, compileDir types.CompileDir) error {
	s := name.String()

	// Step 0: action on what already exists in the in-memory view of the fs.
	if entry, exists := a.DirEntries[s]; exists && !entry.IsDir() {
		// Happy path, let the call-site overwrite the file.
		return nil
	} else if exists {
		// Entry is a directory instead of a file, delete it recursively.
		// Case A: The last compile has replaced the file with a directory.
		// Case B: The user has restructured the tree since the last compile.
		if err := a.DropTree(name, compileDir); err != nil {
			return err
		}
	} else {
		// Make sure all the parents exist and are directories.
		if err := a.EnsureIsDir(name.Dir(), compileDir); err != nil {
			return err
		}
	}

	// Step 1: persist new state in the in-memory view of the fs.
	a.DirEntries[s] = name
	return nil
}
