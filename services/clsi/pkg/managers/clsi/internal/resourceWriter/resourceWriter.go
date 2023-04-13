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
	SyncResourcesToDisk(
		ctx context.Context,
		projectId sharedTypes.UUID,
		namespace types.Namespace,
		request *types.CompileRequest,
	) (ResourceCache, error)

	Clear(projectId sharedTypes.UUID, namespace types.Namespace) error

	GetState(namespace types.Namespace) types.SyncState
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

func (r *resourceWriter) GetState(namespace types.Namespace) types.SyncState {
	s, _ := r.loadResourceCache(namespace)
	return s
}

func (r *resourceWriter) SyncResourcesToDisk(ctx context.Context, projectId sharedTypes.UUID, namespace types.Namespace, request *types.CompileRequest) (ResourceCache, error) {
	dir := r.compileBaseDir.CompileDir(namespace)
	var cache ResourceCache
	var err error
	switch {
	case request.Options.SyncType == types.SyncTypeFull:
		cache, err = r.fullSync(ctx, projectId, request, dir)
	case request.Options.SyncType == types.SyncTypeFullIncremental:
		cache, err = r.fullSyncIncremental(
			ctx, projectId, namespace, request, dir,
		)
	default:
		cache, err = r.incrementalSync(ctx, projectId, namespace, request, dir)
	}
	if err != nil {
		if err != outputFileFinder.ErrProjectHasTooManyFilesAndDirectories {
			return nil, err
		}
		// Clear all the contents.
		if err2 := r.Clear(projectId, namespace); err2 != nil {
			return nil, err
		}
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		// Retry once when doing a fully sync.
		switch request.Options.SyncType {
		case types.SyncTypeFull:
			cache, err = r.fullSync(ctx, projectId, request, dir)
		case types.SyncTypeFullIncremental:
			cache, err = r.fullSyncIncremental(
				ctx, projectId, namespace, request, dir,
			)
		case types.SyncTypeIncremental:
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

func (r *resourceWriter) Clear(projectId sharedTypes.UUID, namespace types.Namespace) error {
	compileDir := r.compileBaseDir.CompileDir(namespace)
	errClearCompileDir := os.RemoveAll(string(compileDir))
	errClearCache := r.urlCache.ClearForProject(projectId)

	if errClearCompileDir != nil && !os.IsNotExist(errClearCompileDir) {
		return errClearCompileDir
	}
	if errClearCache != nil && !os.IsNotExist(errClearCache) {
		return errClearCache
	}
	return nil
}

func (r *resourceWriter) fullSync(ctx context.Context, projectId sharedTypes.UUID, request *types.CompileRequest, dir types.CompileDir) (ResourceCache, error) {
	cache := composeResourceCache(request)

	err := r.sync(ctx, projectId, request, dir, cache)
	if err != nil {
		return nil, err
	}
	return cache, nil
}

func (r *resourceWriter) fullSyncIncremental(ctx context.Context, projectId sharedTypes.UUID, namespace types.Namespace, request *types.CompileRequest, dir types.CompileDir) (ResourceCache, error) {
	cache := composeResourceCache(request)

	err := r.sync(ctx, projectId, request, dir, cache)
	if err != nil {
		return nil, err
	}

	err = r.storeResourceCache(namespace, cache, request.Options.SyncState)
	if err != nil {
		return nil, err
	}
	return cache, nil
}

func (r *resourceWriter) incrementalSync(ctx context.Context, projectId sharedTypes.UUID, namespace types.Namespace, request *types.CompileRequest, dir types.CompileDir) (ResourceCache, error) {
	s, cache := r.loadResourceCache(namespace)
	if s == types.SyncStateCleared {
		return nil, &errors.InvalidStateError{
			Msg: "missing cache for incremental sync",
		}
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	err := r.sync(ctx, projectId, request, dir, cache)
	if err != nil {
		return nil, err
	}

	return cache, nil
}

func (r *resourceWriter) sync(ctx context.Context, projectId sharedTypes.UUID, request *types.CompileRequest, compileDir types.CompileDir, allResources ResourceCache) error {
	if err := r.urlCache.SetupForProject(ctx, projectId); err != nil {
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
					projectId,
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

func (r *resourceWriter) writeResource(ctx context.Context, projectId sharedTypes.UUID, resource *types.Resource, compileDir types.CompileDir) error {
	if resource.IsDoc() {
		return r.writeDoc(resource, compileDir)
	}
	return r.writeFile(ctx, projectId, resource, compileDir)
}

func (r *resourceWriter) writeDoc(resource *types.Resource, compileDir types.CompileDir) error {
	// There is no need for atomic writes here. In the error case, the file
	//  will either get recreated on re-compile, or deleted as part of output
	//  scrubbing in case it were to be deleted from the projects resources.
	blob := []byte(resource.Content)
	return os.WriteFile(compileDir.Join(resource.Path), blob, 0o600)
}

func (r *resourceWriter) writeFile(ctx context.Context, projectId sharedTypes.UUID, resource *types.Resource, compileDir types.CompileDir) error {
	return r.urlCache.Download(ctx, projectId, resource, compileDir)
}
