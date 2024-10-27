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

package outputCache

import (
	"context"
	"os"
	"syscall"

	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/copyFile"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/constants"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi/internal/outputFileFinder"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi/internal/resourceCleanup"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi/internal/resourceWriter"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

type Manager interface {
	ListOutputFiles(ctx context.Context, namespace types.Namespace, buildId types.BuildId) (types.OutputFiles, error)
	SaveOutputFiles(ctx context.Context, allResources resourceWriter.ResourceCache, namespace types.Namespace, buildId types.BuildId, pdfCachingDependencyProcessed chan struct{}) (types.OutputFiles, HasOutputPDF, error)
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

func (m *manager) ListOutputFiles(ctx context.Context, namespace types.Namespace, buildId types.BuildId) (types.OutputFiles, error) {
	outputDir := m.paths.OutputBaseDir.OutputDir(namespace)
	compileOutputDir := outputDir.CompileOutputDir(buildId)
	allFiles, err := m.finder.FindAll(ctx, types.CompileDir(compileOutputDir))
	if err != nil {
		return nil, err
	}

	outputFiles := make(types.OutputFiles, 0, len(allFiles.FileStats))
	for fileName, d := range allFiles.FileStats {
		// Take the file mode from bulk scan results.
		if !d.Type().IsRegular() {
			continue
		}

		file := types.OutputFile{
			Build: buildId,
			DownloadPath: types.BuildDownloadPathFromNamespace(
				namespace, buildId, fileName,
			),
			Path: fileName,
			Type: fileName.Type(),
		}

		if fileName == constants.OutputPDF {
			info, err2 := d.Info()
			if err2 != nil {
				return nil, err2
			}
			file.Size = info.Size()
			file.Ranges = emptyRanges
		}

		outputFiles = append(outputFiles, file)
	}
	return outputFiles, nil
}

type HasOutputPDF bool

var emptyRanges = make([]types.PDFCachingRange, 0)

func (m *manager) SaveOutputFiles(ctx context.Context, allResources resourceWriter.ResourceCache, namespace types.Namespace, buildId types.BuildId, pdfCachingDependencyProcessed chan struct{}) (types.OutputFiles, HasOutputPDF, error) {
	defer close(pdfCachingDependencyProcessed)
	compileDir := m.paths.CompileBaseDir.CompileDir(namespace)
	outputDir := m.paths.OutputBaseDir.OutputDir(namespace)
	compileOutputDir := outputDir.CompileOutputDir(buildId)
	dirHelper := createdDirs{
		base:  compileOutputDir,
		isDir: make(map[sharedTypes.DirName]bool),
	}
	if err := dirHelper.CreateBase(); err != nil {
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
			}

			if fileName == constants.OutputPDF {
				// Fetch the file stats before potentially moving the file.
				info, err2 := d.Info()
				if err2 != nil {
					return err2
				}
				file.Size = info.Size()
				file.Ranges = emptyRanges
				hasOutputPDF = true
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
				dst := compileOutputDir.Join(fileName)
				if resourceCleanup.ShouldDelete(fileName) {
					// Optimization: Steal the file from the compileDir.
					// The next compile request would delete it anyway.
					if err2 := syscall.Rename(src, dst); err2 != nil {
						return errors.Tag(err2, "rename "+src+" -> "+dst)
					}
				} else {
					if err2 := copyFile.NonAtomic(dst, src); err2 != nil {
						return errors.Tag(err2, "copy "+src+" -> "+dst)
					}
				}
				if fileName == constants.OutputPDF ||
					fileName == constants.PDFCachingXrefFilename {
					pdfCachingDependencyProcessed <- struct{}{}
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
