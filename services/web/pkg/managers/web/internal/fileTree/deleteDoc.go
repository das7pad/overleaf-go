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

package fileTree

import (
	"context"
	"time"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) DeleteDocFromProject(ctx context.Context, request *types.DeleteDocRequest) error {
	projectId := request.ProjectId
	userId := request.UserId
	docId := request.DocId

	projectVersion, err := m.pm.DeleteDoc(ctx, projectId, userId, docId)
	if err != nil {
		return err
	}

	// Notify real-time first, triggering users to leave the doc.
	m.notifyEditor(projectId, "removeEntity", deleteTreeElementUpdate{
		EntityId:       docId,
		ProjectVersion: projectVersion,
	})

	// The doc has been deleted.
	// Failing the request and retrying now would result in a 404.
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	m.cleanupDocDeletion(ctx, projectId, docId)
	return nil
}

func (m *manager) cleanupDocDeletion(ctx context.Context, projectId, docId sharedTypes.UUID) {
	// Cleanup in document-updater
	_ = m.dum.FlushAndDeleteDoc(ctx, projectId, docId)
}
