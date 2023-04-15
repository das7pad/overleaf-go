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

package resourceWriter

import (
	"context"
	"os"
	"path"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi/internal/outputFileFinder"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi/internal/resourceCleanup"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi/internal/urlCache"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

type ResourceWriter interface {
	SyncResourcesToDisk(ctx context.Context, namespace types.Namespace, request *types.CompileRequest) (ResourceCache, error)
	CreateCompileDir(namespace types.Namespace) error
	Clear(namespace types.Namespace) error
	HasContent(namespace types.Namespace) bool
}

func New(options *types.Options, finder outputFileFinder.Finder) (ResourceWriter, error) {
	c, err := urlCache.New(options)
	if err != nil {
		return nil, err
	}
	cacheDir := string(options.CacheBaseDir)
	if err = os.MkdirAll(cacheDir, 0o700); err != nil {
		return nil, err
	}
	stateDir := path.Dir(options.CacheBaseDir.StateFile(""))
	if err = os.MkdirAll(stateDir, 0o700); err != nil {
		return nil, err
	}

	return &resourceWriter{
		cacheBaseDir:          options.CacheBaseDir,
		compileBaseDir:        options.CompileBaseDir,
		parallelResourceWrite: options.ParallelResourceWrite,
		finder:                finder,
		urlCache:              c,
	}, nil
}

type resourceWriter struct {
	cacheBaseDir          types.CacheBaseDir
	compileBaseDir        types.CompileBaseDir
	parallelResourceWrite int64

	finder outputFileFinder.Finder

	urlCache urlCache.URLCache
}

func (r *resourceWriter) CreateCompileDir(namespace types.Namespace) error {
	compileDir := string(r.compileBaseDir.CompileDir(namespace))
	if err := os.Mkdir(compileDir, 0o755); err != nil && !os.IsExist(err) {
		return errors.Tag(err, "create compile dir")
	}
	return nil
}

func (r *resourceWriter) HasContent(namespace types.Namespace) bool {
	_, err := os.Stat(r.cacheBaseDir.StateFile(namespace))
	return err == nil
}

func (r *resourceWriter) SyncResourcesToDisk(ctx context.Context, namespace types.Namespace, request *types.CompileRequest) (ResourceCache, error) {
	dir := r.compileBaseDir.CompileDir(namespace)
	var cache ResourceCache
	var err error
	if request.Options.SyncType == types.SyncTypeFullIncremental {
		cache, err = r.fullSyncIncremental(
			ctx, namespace, request, dir,
		)
	} else {
		cache, err = r.incrementalSync(ctx, namespace, request, dir)
	}
	if err != nil {
		if err != outputFileFinder.ErrProjectHasTooManyFilesAndDirectories {
			return nil, err
		}
		// Clear all the contents.
		if err2 := r.Clear(namespace); err2 != nil {
			return nil, err
		}
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		// Retry once when doing a full sync.
		if request.Options.SyncType == types.SyncTypeFullIncremental {
			cache, err = r.fullSyncIncremental(ctx, namespace, request, dir)
		} else {
			// Let web try with a full sync request again.
			return nil, err
		}
		if err != nil {
			return nil, err
		}
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return cache, nil
}

func (r *resourceWriter) Clear(namespace types.Namespace) error {
	compileDir := r.compileBaseDir.CompileDir(namespace)
	errClearState := os.Remove(r.cacheBaseDir.StateFile(namespace))
	errClearCompileDir := os.RemoveAll(string(compileDir))
	errClearURLCache := r.urlCache.ClearForProject(namespace)

	if errClearState != nil && !os.IsNotExist(errClearState) {
		return errClearState
	}
	if errClearCompileDir != nil && !os.IsNotExist(errClearCompileDir) {
		return errClearCompileDir
	}
	if errClearURLCache != nil && !os.IsNotExist(errClearURLCache) {
		return errClearURLCache
	}
	return nil
}

func (r *resourceWriter) fullSyncIncremental(ctx context.Context, namespace types.Namespace, request *types.CompileRequest, dir types.CompileDir) (ResourceCache, error) {
	cache := composeResourceCache(request)

	err := r.sync(ctx, namespace, request, dir, cache)
	if err != nil {
		return nil, err
	}

	err = r.storeResourceCache(namespace, cache, request.Options.SyncState)
	if err != nil {
		return nil, err
	}
	return cache, nil
}

func (r *resourceWriter) incrementalSync(ctx context.Context, namespace types.Namespace, request *types.CompileRequest, dir types.CompileDir) (ResourceCache, error) {
	s, cache := r.loadResourceCache(namespace)
	if s != request.Options.SyncState {
		return nil, &errors.InvalidStateError{
			Msg: "local sync state differs remote state, must perform full sync",
		}
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	err := r.sync(ctx, namespace, request, dir, cache)
	if err != nil {
		return nil, err
	}

	return cache, nil
}

func (r *resourceWriter) sync(ctx context.Context, namespace types.Namespace, request *types.CompileRequest, compileDir types.CompileDir, allResources ResourceCache) error {
	if err := r.urlCache.SetupForProject(namespace); err != nil {
		return err
	}

	eg, workCtx := errgroup.WithContext(ctx)
	concurrency := r.parallelResourceWrite

	work := make(chan *types.Resource, concurrency*3)
	eg.Go(func() error {
		defer close(work)

		// Working with allFiles is very performant, but not thread-safe.
		// Fetch and access it from this fan-out thread only.
		allFiles, finderErr := r.finder.FindAll(workCtx, compileDir)
		if finderErr != nil {
			return finderErr
		}

		aborted := workCtx.Done()
		for _, resource := range request.Resources {
			err := allFiles.EnsureIsWritable(resource.Path, compileDir)
			if err != nil {
				return err
			}
			select {
			case work <- resource:
				continue
			case <-aborted:
				return workCtx.Err()
			}
		}
		foundResources := 0
		for _, entry := range allFiles.DirEntries {
			fileName, isFile := entry.(sharedTypes.PathName)
			if !isFile {
				continue
			}
			if _, foundResource := allResources[fileName]; foundResource {
				foundResources++
				continue
			}
			if !resourceCleanup.ShouldDelete(fileName) {
				continue
			}
			aliasResource := request.RootDocAliasResource
			if aliasResource != nil && aliasResource.Path == fileName {
				continue
			}
			if err := allFiles.Delete(fileName, compileDir); err != nil {
				return err
			}
			// Check context on slow path only.
			if err := workCtx.Err(); err != nil {
				return err
			}
		}
		if foundResources != len(allResources) {
			missing := make([]string, 0)
			for fileName := range allResources {
				s := fileName.String()
				entry, exists := allFiles.DirEntries[s]
				if exists && !entry.IsDir() {
					continue
				}
				missing = append(missing, s)
			}
			flat := strings.Join(missing, ", ")
			return &errors.InvalidStateError{Msg: "missing files: " + flat}
		}
		return nil
	})

	for i := int64(0); i < concurrency; i++ {
		eg.Go(func() error {
			for resource := range work {
				err := r.writeResource(
					workCtx,
					namespace,
					resource,
					compileDir,
				)
				if err != nil {
					p := resource.Path.String()
					return errors.Tag(err, "write failed for "+p)
				}
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		for range work {
			// Flush the queue.
		}
		return err
	}
	return nil
}

func (r *resourceWriter) writeResource(ctx context.Context, namespace types.Namespace, resource *types.Resource, compileDir types.CompileDir) error {
	if resource.IsDoc() {
		return r.writeDoc(resource, compileDir)
	}
	return r.writeFile(ctx, namespace, resource, compileDir)
}

func (r *resourceWriter) writeDoc(resource *types.Resource, compileDir types.CompileDir) error {
	// There is no need for atomic writes here. In the error case, the file
	//  will either get recreated on re-compile, or deleted as part of output
	//  scrubbing in case it were to be deleted from the projects resources.
	blob := []byte(resource.Content)
	return os.WriteFile(compileDir.Join(resource.Path), blob, 0o600)
}

func (r *resourceWriter) writeFile(ctx context.Context, namespace types.Namespace, resource *types.Resource, compileDir types.CompileDir) error {
	return r.urlCache.Download(ctx, namespace, resource, compileDir)
}
