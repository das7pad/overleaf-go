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
	"bytes"
	"encoding/json"
	"os"

	"github.com/das7pad/overleaf-go/pkg/copyFile"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

type ResourceCache map[sharedTypes.PathName]struct{}
type projectState struct {
	FlatResourceCache []sharedTypes.PathName
	SyncState         types.SyncState
}

func (r *resourceWriter) loadResourceCache(namespace types.Namespace) (types.SyncState, ResourceCache) {
	file, err := os.Open(r.cacheBaseDir.StateFile(namespace))
	if err != nil {
		return types.SyncStateCleared, nil
	}
	defer func() {
		_ = file.Close()
	}()
	s := projectState{}
	if err = json.NewDecoder(file).Decode(&s); err != nil {
		return types.SyncStateCleared, nil
	}
	cache := make(ResourceCache, len(s.FlatResourceCache))
	for _, p := range s.FlatResourceCache {
		cache[p] = struct{}{}
	}
	return s.SyncState, cache
}

func composeResourceCache(request *types.CompileRequest) ResourceCache {
	cache := make(ResourceCache, len(request.Resources))
	for _, resource := range request.Resources {
		if resource == request.RootDocAliasResource {
			continue
		}
		cache[resource.Path] = struct{}{}
	}
	return cache
}

func (r *resourceWriter) storeResourceCache(namespace types.Namespace, cache ResourceCache, syncState types.SyncState) error {
	s := projectState{
		FlatResourceCache: make([]sharedTypes.PathName, 0, len(cache)),
		SyncState:         syncState,
	}
	for p := range cache {
		s.FlatResourceCache = append(s.FlatResourceCache, p)
	}
	buf := bytes.Buffer{}
	buf.Grow(10 * 1024)
	if err := json.NewEncoder(&buf).Encode(s); err != nil {
		return err
	}
	return copyFile.Atomic(r.cacheBaseDir.StateFile(namespace), &buf)
}
