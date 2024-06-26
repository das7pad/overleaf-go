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

package urlCache

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/das7pad/overleaf-go/pkg/copyFile"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

type URLCache interface {
	SetupForProject(namespace types.Namespace) error
	Download(ctx context.Context, namespace types.Namespace, resource *types.Resource, dir types.CompileDir) error
	ClearForProject(namespace types.Namespace) error
}

func New(options *types.Options) (URLCache, error) {
	cacheDir := options.CacheBaseDir
	if err := os.MkdirAll(string(cacheDir), 0o700); err != nil {
		return nil, err
	}

	return &urlCache{
		cacheDir: cacheDir,
		tries:    1 + options.URLDownloadRetries,

		client: http.Client{
			Timeout: options.URLDownloadTimeout,
		},
	}, nil
}

type urlCache struct {
	cacheDir types.CacheBaseDir
	tries    int64

	client http.Client
}

func (u *urlCache) SetupForProject(namespace types.Namespace) error {
	err := os.Mkdir(string(u.projectDir(namespace)), 0o700)
	if err == nil || os.IsExist(err) {
		return nil
	}
	return err
}

func (u *urlCache) projectDir(namespace types.Namespace) types.ProjectCacheDir {
	return u.cacheDir.ProjectCacheDir(namespace)
}

func (u *urlCache) downloadIntoCache(ctx context.Context, url sharedTypes.URL, cachePath string) error {
	r, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		url.String(),
		nil,
	)
	if err != nil {
		return err
	}
	response, err := u.client.Do(r)
	if err != nil {
		return err
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if response.StatusCode != http.StatusOK {
		return errors.New(
			"file download returned non success: status " + response.Status,
		)
	}
	return copyFile.Atomic(cachePath, response.Body)
}

func (u *urlCache) downloadIntoCacheWithRetries(ctx context.Context, url sharedTypes.URL, cachePath string) error {
	var err error
	for i := int64(0); i < u.tries; i++ {
		if err = u.downloadIntoCache(ctx, url, cachePath); err == nil {
			return nil
		}
		if err2 := ctx.Err(); err2 != nil {
			return err2
		}
	}
	return err
}

func (u *urlCache) Download(ctx context.Context, namespace types.Namespace, resource *types.Resource, dir types.CompileDir) error {
	cachePath := u.projectDir(namespace).Join(
		sharedTypes.PathName(strings.ReplaceAll(resource.URL.Path, "/", "-")),
	)
	dst := dir.Join(resource.Path)

	// There is no need for atomic writes here. In the error case, the file
	//  will either get recreated on re-compile, or deleted as part of output
	//  scrubbing in case it were to be deleted from the projects resources for
	//  the next compile.
	if err := copyFile.NonAtomic(dst, cachePath); err == nil {
		// Happy path
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	err := u.downloadIntoCacheWithRetries(ctx, *resource.URL, cachePath)
	if err != nil {
		return err
	}

	// See the note above why we do not need atomic copying here.
	if err = copyFile.NonAtomic(dst, cachePath); err != nil {
		return err
	}
	return nil
}

func (u *urlCache) ClearForProject(namespace types.Namespace) error {
	return os.RemoveAll(string(u.projectDir(namespace)))
}
