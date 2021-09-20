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

package outputCache

import (
	"context"
	"encoding/hex"
	"math/rand"
	"os"
	"strconv"
	"syscall"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/constants"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/copyFile"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi/internal/outputFileFinder"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi/internal/resourceCleanup"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi/internal/resourceWriter"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

type Manager interface {
	SaveOutputFiles(
		ctx context.Context,
		files *types.CommandOutputFiles,
		allResources resourceWriter.ResourceCache,
		namespace types.Namespace,
	) (types.OutputFiles, HasOutputPDF, error)

	Clear(namespace types.Namespace) error
}

func New(options *types.Options, finder outputFileFinder.Finder) Manager {
	return &manager{
		finder:  finder,
		options: options,
	}
}

type manager struct {
	finder  outputFileFinder.Finder
	options *types.Options
}

type HasOutputPDF bool

func (m *manager) SaveOutputFiles(ctx context.Context, files *types.CommandOutputFiles, allResources resourceWriter.ResourceCache, namespace types.Namespace) (types.OutputFiles, HasOutputPDF, error) {
	buildId, err := getBuildId()
	if err != nil {
		return nil, false, err
	}
	compileDir := m.options.CompileBaseDir.CompileDir(namespace)
	outputDir := m.options.OutputBaseDir.OutputDir(namespace)
	compileOutputDir := outputDir.CompileOutputDir(buildId)
	dirHelper := createdDirs{
		base:  compileOutputDir,
		isDir: make(map[sharedTypes.DirName]bool),
	}
	if err = dirHelper.CreateBase(); err != nil {
		return nil, false, err
	}

	allFiles, err := m.finder.FindAll(ctx, compileDir)
	if err != nil {
		return nil, false, err
	}
	outputFiles := make(types.OutputFiles, 0)
	hasOutputPDF := HasOutputPDF(false)
	for fileName, d := range allFiles.FileStats {
		if _, isResource := allResources[fileName]; isResource {
			continue
		}

		// Take the file mode from bulk scan results.
		if !d.Type().IsRegular() {
			continue
		}

		var size int64
		if fileName == "output.pdf" {
			// Fetch the file stats before potentially moving the file.
			if info, err2 := d.Info(); err2 != nil {
				return nil, false, err2
			} else {
				size = info.Size()
			}
			hasOutputPDF = true
		}

		destFileName := fileName
		if fileName == files.StdErr {
			destFileName = "output.stderr"
		} else if fileName == files.StdOut {
			destFileName = "output.stdout"
		}

		if err = dirHelper.EnsureIsWritable(destFileName); err != nil {
			return nil, false, err
		}

		src := compileDir.Join(fileName)
		dest := compileOutputDir.Join(destFileName)
		if resourceCleanup.ShouldDelete(fileName) {
			// Optimization: Steal the file from the compileDir.
			// The next compile request would delete it anyways.
			if err = syscall.Rename(src, dest); err != nil {
				return nil, false, errors.Tag(
					err, "cannot rename "+src+" -> "+dest,
				)
			}
		} else {
			if err = copyFile.NonAtomic(src, dest); err != nil {
				return nil, false, err
			}
		}

		file := types.OutputFile{
			Build:        buildId,
			DownloadPath: getDownloadPath(namespace, buildId, destFileName),
			Path:         destFileName,
			Type:         destFileName.Type(),
			Size:         size,
		}
		outputFiles = append(outputFiles, file)
	}
	cleanupExpired(outputDir, buildId, namespace)
	return outputFiles, hasOutputPDF, nil
}

func (m *manager) Clear(namespace types.Namespace) error {
	outputDir := m.options.OutputBaseDir.OutputDir(namespace)
	return os.RemoveAll(string(outputDir))
}

// getBuildId yields a secure unique id
// It contains a 16 hex char long timestamp in ns precision, a hyphen and
//  another 16 hex char long random string.
func getBuildId() (types.BuildId, error) {
	buf := make([]byte, 8)
	_, err := rand.Read(buf)
	if err != nil {
		return "", err
	}
	now := time.Now().UnixNano()
	buildId := types.BuildId(
		strconv.FormatInt(now, 16) + "-" + hex.EncodeToString(buf),
	)
	return buildId, nil
}

const publicProjectPrefix = "/project"

func getDownloadPath(namespace types.Namespace, id types.BuildId, name sharedTypes.PathName) types.DownloadPath {
	return types.DownloadPath(
		publicProjectPrefix +
			"/" + string(namespace) +
			"/" + constants.CompileOutputLabel +
			"/" + string(id) +
			"/" + string(name),
	)
}
