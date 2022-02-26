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

package projectMetadata

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/edgedb/edgedb-go"
	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	documentUpdaterTypes "github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

const cacheExpiry = time.Hour * 24

type cacheEntry struct {
	ProjectVersion time.Time                  `json:"projectVersion"`
	ProjectMeta    types.LightProjectMetadata `json:"projectMeta"`
}

func getCacheKey(projectId edgedb.UUID) string {
	return "metadata:" + projectId.String()
}

func (m *manager) getCacheEntry(ctx context.Context, projectId edgedb.UUID) (*cacheEntry, error) {
	raw, err := m.client.Get(ctx, getCacheKey(projectId)).Bytes()
	if err != nil {
		return nil, err
	}
	entry := &cacheEntry{}
	if err = json.Unmarshal(raw, entry); err != nil {
		return nil, err
	}
	return entry, nil
}

func (m *manager) setCacheEntry(ctx context.Context, projectId edgedb.UUID, entry *cacheEntry) error {
	blob, err := json.Marshal(entry)
	if err != nil {
		return errors.Tag(err, "cannot serialize cache entry")
	}
	err = m.client.Set(ctx, getCacheKey(projectId), blob, cacheExpiry).Err()
	if err != nil {
		return errors.Tag(err, "cannot populate cache entry")
	}
	return nil
}

func (m *manager) getForProjectWithCache(ctx context.Context, projectId edgedb.UUID) (types.LightProjectMetadata, error) {
	var cached *cacheEntry
	var projectVersionFlushed time.Time
	var recentlyEdited documentUpdaterTypes.DocContentSnapshots
	eg, pCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		cached, _ = m.getCacheEntry(pCtx, projectId)
		return nil
	})
	eg.Go(func() error {
		p := &project.LastUpdatedAtField{}
		if err := m.pm.GetProject(pCtx, projectId, p); err != nil {
			return errors.Tag(err, "cannot get project from mongo")
		}
		projectVersionFlushed = p.LastUpdatedAt
		return nil
	})
	eg.Go(func() error {
		docs, err := m.dum.GetProjectDocsAndFlushIfOldSnapshot(
			pCtx,
			projectId,
		)
		if err != nil {
			return errors.Tag(err, "cannot get docs from redis")
		}
		recentlyEdited = docs
		return nil
	})
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	projectVersionLive := recentlyEdited.LastUpdatedAt()
	projectVersion := projectVersionFlushed
	if projectVersionLive.After(projectVersionFlushed) {
		projectVersion = projectVersionLive
	}
	if cached != nil && cached.ProjectVersion.Equal(projectVersion) {
		return cached.ProjectMeta, nil
	}

	meta, err := m.getForProjectWithoutCache(ctx, projectId, recentlyEdited)
	if err != nil {
		return nil, err
	}

	go func() {
		cached = &cacheEntry{
			ProjectVersion: projectVersion,
			ProjectMeta:    meta,
		}
		ctx2, done := context.WithTimeout(context.Background(), time.Second*10)
		defer done()
		if err2 := m.setCacheEntry(ctx2, projectId, cached); err2 != nil {
			log.Println(errors.Tag(err2, projectId.String()).Error())
		}
	}()
	return meta, nil
}
