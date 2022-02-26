// Golang port of Overleaf
// Copyright (C) 2021-2022 Jakob Ackermann <das7pad@outlook.com>
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

package webApi

import (
	"context"
	"time"

	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/services/docstore/pkg/managers/docstore"

	"github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
)

type monolithManager struct {
	dm docstore.Manager
	pm project.Manager
}

func (m *monolithManager) GetDoc(ctx context.Context, projectId, docId edgedb.UUID) (*types.FlushedDoc, error) {
	_, p, err := m.pm.GetDocMeta(ctx, projectId, docId)
	if err != nil {
		if errors.IsNotFoundError(err) {
			isDeleted, err2 := m.dm.IsDocDeleted(ctx, projectId, docId)
			if err2 != nil {
				return nil, errors.Tag(err2, "cannot check is doc deleted")
			}
			if isDeleted {
				// 404 when requesting deleted doc.
				return nil, err
			}
			// 403 when requesting doc from other project.
			return nil, &errors.NotAuthorizedError{}
		}
		return nil, errors.Tag(err, "cannot get doc path")
	}
	d, err := m.dm.GetFullDoc(ctx, projectId, docId)
	if err != nil {
		return nil, errors.Tag(err, "cannot get doc from mongo")
	}
	if d.Deleted {
		// The doc could have been deleted in the meantime.
		return nil, &errors.ErrorDocNotFound{}
	}
	doc := &types.FlushedDoc{
		Lines:    d.Lines,
		PathName: p,
		Ranges:   d.Ranges,
		Version:  d.Version,
	}
	return doc, nil
}

func (m *monolithManager) SetDoc(ctx context.Context, projectId, docId edgedb.UUID, doc *types.SetDocDetails) error {
	modified, err := m.dm.UpdateDoc(ctx, projectId, docId, doc.ForDocUpdate)
	if err != nil {
		return errors.Tag(err, "cannot set doc in mongo")
	}
	if modified {
		at := time.Unix(doc.LastUpdatedAt, 0)
		err = m.pm.UpdateLastUpdated(ctx, projectId, at, doc.LastUpdatedBy)
		if err != nil {
			return errors.Tag(err, "cannot update project context")
		}
	}
	return nil
}
