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
	"encoding/json"
	"os"

	"github.com/das7pad/overleaf-go/services/clsi/pkg/constants"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

const StateFile = types.FileName(constants.ProjectSyncStateFilename)

func (r *resourceWriter) getStatePath(namespace types.Namespace) string {
	return r.options.CacheBaseDir.NamespacedCacheDir(namespace).Join(StateFile)
}

type resourceCacheEntry struct {
	Path       types.FileName    `json:"p"`
	ModifiedAt *types.ModifiedAt `json:"m"`
	URL        *types.URL        `json:"u"`
}

type ResourceCache map[types.FileName]resourceCacheEntry

func (r *resourceWriter) loadResourceCache(namespace types.Namespace) ResourceCache {
	cache := make(ResourceCache)
	file, err := os.Open(r.getStatePath(namespace))
	if err != nil {
		return cache
	}
	defer func() {
		_ = file.Close()
	}()
	if err = json.NewDecoder(file).Decode(&cache); err != nil {
		return cache
	}
	return cache
}

func composeResourceCache(request *types.CompileRequest) ResourceCache {
	cache := make(ResourceCache, len(request.Resources))
	for _, resource := range request.Resources {
		if resource == request.RootDocAliasResource {
			continue
		}
		cache[resource.Path] = resourceCacheEntry{
			Path:       resource.Path,
			ModifiedAt: resource.ModifiedAt,
			URL:        resource.URL,
		}
	}
	return cache
}

func (r *resourceWriter) storeResourceCache(namespace types.Namespace, cache ResourceCache) error {
	target := r.getStatePath(namespace)
	tmp := target + "~"
	file, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()
	if err = json.NewEncoder(file).Encode(cache); err != nil {
		return err
	}
	if err = os.Rename(tmp, target); err != nil {
		return err
	}
	return nil
}
