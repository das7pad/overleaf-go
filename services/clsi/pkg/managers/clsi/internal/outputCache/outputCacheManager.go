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

package outputCache

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"strconv"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/copyFile"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi/internal/outputFileFinder"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi/internal/resourceCleanup"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi/internal/resourceWriter"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

type Manager interface {
	SaveOutputFiles(ctx context.Context, allResources resourceWriter.ResourceCache, namespace types.Namespace) (types.OutputFiles, HasOutputPDF, error)
	Clear(namespace types.Namespace) error
}

func New(options *types.Options, finder outputFileFinder.Finder) Manager {
	return &manager{
		finder:              finder,
		paths:               options.Paths,
		parallelOutputWrite: options.ParallelOutputWrite,
	}
}

type manager struct {
	finder              outputFileFinder.Finder
	paths               types.Paths
	parallelOutputWrite int64
}

type HasOutputPDF bool

func (m *manager) SaveOutputFiles(ctx context.Context, allResources resourceWriter.ResourceCache, namespace types.Namespace) (types.OutputFiles, HasOutputPDF, error) {
	buildId, err := getBuildId()
	if err != nil {
		return nil, false, err
	}
	compileDir := m.paths.CompileBaseDir.CompileDir(namespace)
	outputDir := m.paths.OutputBaseDir.OutputDir(namespace)
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

	eg, pCtx := errgroup.WithContext(ctx)
	concurrency := m.parallelOutputWrite
	work := make(chan sharedTypes.PathName, 3*concurrency)
	outputFiles := make(types.OutputFiles, 0)
	hasOutputPDF := HasOutputPDF(false)
	eg.Go(func() error {
		defer close(work)
		aborted := pCtx.Done()
		for fileName, d := range allFiles.FileStats {
			if _, isResource := allResources[fileName]; isResource {
				continue
			}

			// Take the file mode from bulk scan results.
			if !d.Type().IsRegular() {
				continue
			}

			if err2 := pCtx.Err(); err2 != nil {
				return err2
			}

			var size int64
			if fileName == "output.pdf" {
				// Fetch the file stats before potentially moving the file.
				info, err2 := d.Info()
				if err2 != nil {
					return err2
				}
				size = info.Size()
				hasOutputPDF = true
			}

			if err2 := dirHelper.EnsureIsWritable(fileName); err2 != nil {
				return err2
			}

			file := types.OutputFile{
				Build: buildId,
				DownloadPath: types.BuildDownloadPathFromNamespace(
					namespace, buildId, fileName,
				),
				Path: fileName,
				Type: fileName.Type(),
				Size: size,
			}
			outputFiles = append(outputFiles, file)
			select {
			case work <- fileName:
				continue
			case <-aborted:
				return pCtx.Err()
			}
		}
		return nil
	})

	for i := int64(0); i < concurrency; i++ {
		eg.Go(func() error {
			for fileName := range work {
				src := compileDir.Join(fileName)
				dest := compileOutputDir.Join(fileName)
				if resourceCleanup.ShouldDelete(fileName) {
					// Optimization: Steal the file from the compileDir.
					// The next compile request would delete it anyway.
					if err2 := syscall.Rename(src, dest); err2 != nil {
						return errors.Tag(
							err2, "cannot rename "+src+" -> "+dest,
						)
					}
				} else {
					if err2 := copyFile.NonAtomic(dest, src); err2 != nil {
						return errors.Tag(err2, "cannot copy "+src+" -> "+dest)
					}
				}
			}
			return nil
		})
	}
	if err = eg.Wait(); err != nil {
		return nil, false, err
	}
	cleanupExpired(outputDir, buildId, namespace)
	return outputFiles, hasOutputPDF, nil
}

func (m *manager) Clear(namespace types.Namespace) error {
	outputDir := m.paths.OutputBaseDir.OutputDir(namespace)
	return os.RemoveAll(string(outputDir))
}

// getBuildId yields a secure unique id
// It contains a 16 hex char long timestamp in ns precision, a hyphen and
// another 16 hex char long random string.
func getBuildId() (types.BuildId, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	now := time.Now().UnixNano()
	buildId := types.BuildId(
		strconv.FormatInt(now, 16) + "-" + hex.EncodeToString(buf),
	)
	return buildId, nil
}
