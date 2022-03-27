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

	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/doc"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/services/docstore/pkg/managers/docstore"

	"github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
)

type monolithManager struct {
	dm docstore.Manager
	pm project.Manager
}

func (m *monolithManager) GetDoc(ctx context.Context, projectId, docId edgedb.UUID) (*types.FlushedDoc, error) {
	d, p, err := m.pm.GetDoc(ctx, projectId, docId)
	if err != nil {
		return nil, errors.Tag(err, "cannot get doc")
	}
	return &types.FlushedDoc{
		Snapshot: d.Snapshot,
		PathName: p,
		Version:  d.Version,
	}, nil
}

func (m *monolithManager) SetDoc(ctx context.Context, projectId, docId edgedb.UUID, d *doc.ForDocUpdate) error {
	_, err := m.dm.UpdateDoc(ctx, projectId, docId, d)
	if err != nil {
		return errors.Tag(err, "cannot set doc in mongo")
	}
	return nil
}
