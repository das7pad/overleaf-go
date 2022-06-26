// Golang port of Overleaf
// Copyright (C) 2022 Jakob Ackermann <das7pad@outlook.com>
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

package diff

import (
	"context"

	"github.com/das7pad/overleaf-go/pkg/errors"
	documentUpdaterTypes "github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
	"github.com/das7pad/overleaf-go/services/track-changes/pkg/types"
)

func (m *manager) RestoreDocVersion(ctx context.Context, r *types.RestoreDocVersionRequest) error {
	projectId := r.ProjectId
	docId := r.DocId

	s, _, err := m.getDocFrom(ctx, projectId, r.UserId, docId, r.FromV, -1)
	if err != nil {
		return errors.Tag(err, "cannot get old doc version")
	}

	err = m.dum.SetDoc(ctx, projectId, docId, &documentUpdaterTypes.SetDocRequest{
		Snapshot: s,
		Source:   "restore",
		UserId:   r.UserId,
	})
	if err != nil {
		return errors.Tag(err, "cannot persist restored doc version")
	}
	return nil
}
