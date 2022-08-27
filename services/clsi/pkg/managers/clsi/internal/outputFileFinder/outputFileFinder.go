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

package outputFileFinder

import (
	"context"
	"io/fs"
	"path/filepath"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/constants"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

type Finder interface {
	FindAll(
		ctx context.Context,
		dir types.CompileDir,
	) (*AllFilesAndDirs, error)
}

func New(options *types.Options) Finder {
	return &finder{
		maxFilesAndDirsPerProject: options.MaxFilesAndDirsPerProject,
	}
}

type finder struct {
	maxFilesAndDirsPerProject int64
}

var ErrProjectHasTooManyFilesAndDirectories = &errors.InvalidStateError{
	Msg: "project has too many files/directories",
}

func (f *finder) FindAll(ctx context.Context, dir types.CompileDir) (*AllFilesAndDirs, error) {
	dirEntries := make(dirEntriesMap)
	fileStats := make(fileStatsMap)
	parent := string(dir)
	parentLength := len(parent) + 1
	maxEntries := f.maxFilesAndDirsPerProject
	nEntries := int64(0)
	err := filepath.WalkDir(parent, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		nEntries++
		if nEntries > maxEntries {
			return ErrProjectHasTooManyFilesAndDirectories
		}
		if nEntries%100 == 0 {
			if err = ctx.Err(); err != nil {
				return err
			}
		}
		if path == parent {
			// Omit the parent dir
			return nil
		}
		relativePath := path[parentLength:]
		if relativePath == constants.AgentSocketName {
			return nil
		}
		if d.IsDir() {
			dirEntries[relativePath] = sharedTypes.DirName(relativePath)
		} else {
			fileName := sharedTypes.PathName(relativePath)
			dirEntries[relativePath] = fileName
			fileStats[fileName] = d
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return &AllFilesAndDirs{DirEntries: dirEntries, FileStats: fileStats}, nil
}
