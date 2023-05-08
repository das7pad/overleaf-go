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

package learn

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) fillImageCache() error {
	m.imageMux.Lock()
	defer m.imageMux.Unlock()
	root := m.baseImagePath.String()
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if root == p {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		m.imageCache[p] = info.ModTime()
		return nil
	})
	if err != nil {
		return errors.Tag(err, "fill image cache")
	}
	return nil
}

func (m *manager) sweepImageCache() error {
	m.imageMux.Lock()
	defer m.imageMux.Unlock()
	mergedErr := &errors.MergedError{}
	now := time.Now()
	for p, validUntil := range m.imageCache {
		if validUntil.Before(now) {
			delete(m.imageCache, p)
			if err := os.Remove(p); err != nil {
				mergedErr.Add(errors.Tag(err, p))
			}
		}
	}
	return mergedErr.Finalize()
}

func (m *manager) ProxyImage(ctx context.Context, request *types.LearnImageRequest, response *types.LearnImageResponse) error {
	if err := request.Path.Validate(); err != nil {
		return err
	}
	now := time.Now()
	target, fetchedAt, err := m.getImage(ctx, request, now)
	if err != nil {
		return err
	}
	if fetchedAt.Equal(now) {
		response.Age = -1
	} else {
		response.Age = int64(now.Sub(fetchedAt).Seconds())
	}
	response.FSPath = target
	return nil
}

func (m *manager) getImage(ctx context.Context, request *types.LearnImageRequest, now time.Time) (string, time.Time, error) {
	flatPath := strings.ReplaceAll(request.Path.String(), "/", "-")
	target := m.baseImagePath.JoinPath(sharedTypes.PathName(flatPath)).String()
	m.imageMux.RLock()
	fetchedAt, exists := m.imageCache[target]
	m.imageMux.RUnlock()
	if exists && fetchedAt.Add(m.cacheDuration).After(now) {
		return target, fetchedAt, nil
	}

	u := m.baseImageURL.WithPath(request.Path.String())
	f, err := m.proxy.DownloadFile(ctx, u)
	if err != nil {
		if exists {
			// fallback to cache
			return target, fetchedAt, nil
		}
		return "", time.Time{}, errors.Tag(err, "download")
	}
	if err = f.Move(target); err != nil {
		f.Cleanup()
		if exists {
			// fallback to cache
			return target, fetchedAt, nil
		}
		return "", time.Time{}, errors.Tag(err, "move target")
	}
	m.imageMux.Lock()
	m.imageCache[target] = now
	m.imageMux.Unlock()
	return target, now, nil
}
