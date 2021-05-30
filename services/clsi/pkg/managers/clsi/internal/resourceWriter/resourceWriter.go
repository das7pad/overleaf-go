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

package resourceWriter

import (
	"context"
	"fmt"
	"os"
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/clsi/pkg/errors"
	"github.com/das7pad/clsi/pkg/managers/clsi/internal/outputFileFinder"
	"github.com/das7pad/clsi/pkg/managers/clsi/internal/resourceCleanup"
	"github.com/das7pad/clsi/pkg/managers/clsi/internal/urlCache"
	"github.com/das7pad/clsi/pkg/types"
)

type ResourceWriter interface {
	SyncResourcesToDisk(
		ctx context.Context,
		projectId primitive.ObjectID,
		namespace types.Namespace,
		request *types.CompileRequest,
	) (*ResourceCache, error)

	Clear(projectId primitive.ObjectID, namespace types.Namespace) error

	GetState(namespace types.Namespace) (types.SyncState, error)
}

func New(options *types.Options, finder outputFileFinder.Finder) (ResourceWriter, error) {
	c, err := urlCache.New(options)
	if err != nil {
		return nil, err
	}
	cacheDir := string(options.CacheBaseDir)
	if err = os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, err
	}

	return &resourceWriter{
		options:  options,
		finder:   finder,
		urlCache: c,
	}, nil
}

type resourceWriter struct {
	options *types.Options

	finder outputFileFinder.Finder

	urlCache urlCache.URLCache
}

func (r *resourceWriter) GetState(namespace types.Namespace) (types.SyncState, error) {
	p := r.getStatePath(namespace)
	if _, err := os.Stat(p); err != nil {
		if os.IsNotExist(err) {
			return types.SyncStateCleared, nil
		}
		return "", err
	}
	return "", nil
}

func (r *resourceWriter) SyncResourcesToDisk(ctx context.Context, projectId primitive.ObjectID, namespace types.Namespace, request *types.CompileRequest) (*ResourceCache, error) {
	dir := r.options.CompileBaseDir.CompileDir(namespace)
	var cache *ResourceCache
	var err error
	if request.Options.SyncType == types.SyncTypeFull {
		cache, err = r.fullSync(ctx, projectId, request, dir)
	} else if request.Options.SyncType == types.SyncTypeFullIncremental {
		cache, err = r.fullSyncIncremental(
			ctx, projectId, namespace, request, dir,
		)
	} else {
		cache, err = r.incrementalSync(ctx, projectId, namespace, request, dir)
	}
	if err != nil {
		if err != errors.ProjectHasTooManyFilesAndDirectories {
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
		if request.Options.SyncType == types.SyncTypeFull {
			cache, err = r.fullSync(ctx, projectId, request, dir)
		} else if request.Options.SyncType == types.SyncTypeFullIncremental {
			cache, err = r.fullSyncIncremental(
				ctx, projectId, namespace, request, dir,
			)
		} else {
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

func (r *resourceWriter) Clear(projectId primitive.ObjectID, namespace types.Namespace) error {
	compileDir := r.options.CompileBaseDir.CompileDir(namespace)
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

func (r *resourceWriter) fullSync(ctx context.Context, projectId primitive.ObjectID, request *types.CompileRequest, dir types.CompileDir) (*ResourceCache, error) {
	cache := composeResourceCache(request)

	err := r.sync(ctx, projectId, request, dir, cache)
	if err != nil {
		return nil, err
	}
	return cache, nil
}

func (r *resourceWriter) fullSyncIncremental(ctx context.Context, projectId primitive.ObjectID, namespace types.Namespace, request *types.CompileRequest, dir types.CompileDir) (*ResourceCache, error) {
	cache := composeResourceCache(request)

	err := r.sync(ctx, projectId, request, dir, cache)
	if err != nil {
		return nil, err
	}

	err = r.storeResourceCache(namespace, cache)
	if err != nil {
		return nil, err
	}
	return cache, nil
}

func (r *resourceWriter) incrementalSync(ctx context.Context, projectId primitive.ObjectID, namespace types.Namespace, request *types.CompileRequest, dir types.CompileDir) (*ResourceCache, error) {
	cache := r.loadResourceCache(namespace)
	if len(*cache) == 0 {
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

func (r *resourceWriter) sync(ctx context.Context, projectId primitive.ObjectID, request *types.CompileRequest, compileDir types.CompileDir, cache *ResourceCache) error {
	if err := r.urlCache.SetupForProject(ctx, projectId); err != nil {
		return err
	}

	workCtx, cancelWork := context.WithCancel(ctx)
	defer cancelWork()

	var triggerError error
	setErr := func(err error) {
		if triggerError == nil {
			triggerError = err
		}
		cancelWork()
	}

	concurrency := r.options.ParallelResourceWrite

	work := make(chan *types.Resource, concurrency)
	cleanupWork := make(chan types.FileName, concurrency)
	go func() {
		defer close(work)
		defer close(cleanupWork)

		// Working with allFiles is very performant, but not thread-safe.
		// Fetch and access it from this fan-out thread only.
		allFiles, finderErr := r.finder.FindAll(workCtx, compileDir)
		if finderErr != nil {
			setErr(finderErr)
			return
		}

		for _, resource := range request.Resources {
			err := allFiles.EnsureIsWritable(resource.Path, compileDir)
			if err != nil {
				setErr(err)
				return
			}
			select {
			case work <- resource:
				continue
			case <-workCtx.Done():
				setErr(workCtx.Err())
				return
			}
		}
		foundResources := 0
		allResources := *cache
		for fileName, isDir := range allFiles.IsDir {
			if isDir {
				continue
			}
			if _, foundResource := allResources[fileName]; foundResource {
				foundResources += 1
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
				setErr(err)
				return
			}
			// Check context on slow path only.
			if err := workCtx.Err(); err != nil {
				setErr(err)
				return
			}
		}
		if foundResources != len(allResources) {
			missing := make([]string, 0)
			for file := range allResources {
				if isDir, exists := allFiles.IsDir[file]; !exists || isDir {
					missing = append(missing, string(file))
				}
			}
			flat := strings.Join(missing, ", ")
			setErr(&errors.InvalidStateError{
				Msg: "missing files: " + flat,
			})
			return
		}
	}()

	workDone := make(chan bool)
	for i := int64(0); i < concurrency; i++ {
		go func() {
			defer func() {
				workDone <- true
			}()
			for resource := range work {
				err := r.writeResource(
					workCtx,
					projectId,
					resource,
					compileDir,
				)
				if err != nil {
					setErr(
						fmt.Errorf(
							"write failed for %s: %w",
							resource.Path,
							err,
						),
					)
					return
				}
			}
		}()
	}

	for i := int64(0); i < concurrency; i++ {
		<-workDone
	}
	return triggerError
}

func (r *resourceWriter) writeResource(ctx context.Context, projectId primitive.ObjectID, resource *types.Resource, compileDir types.CompileDir) error {
	if resource.IsDoc() {
		return r.writeDoc(resource, compileDir)
	} else {
		return r.writeFile(ctx, projectId, resource, compileDir)
	}
}

func (r *resourceWriter) writeDoc(resource *types.Resource, compileDir types.CompileDir) error {
	// There is no need for atomic writes here. In the error case, the file
	//  will either get recreated on re-compile, or deleted as part of output
	//  scrubbing in case it were to be deleted from the projects resources.
	blob := []byte(*resource.Content)
	return os.WriteFile(compileDir.Join(resource.Path), blob, 0644)
}

func (r *resourceWriter) writeFile(ctx context.Context, projectId primitive.ObjectID, resource *types.Resource, compileDir types.CompileDir) error {
	return r.urlCache.Download(ctx, projectId, resource, compileDir)
}
