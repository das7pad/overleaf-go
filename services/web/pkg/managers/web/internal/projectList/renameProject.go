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

package projectList

import (
	"context"
	"encoding/json"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) RenameProject(ctx context.Context, request *types.RenameProjectRequest) error {
	userId := request.Session.User.Id
	projectId := request.ProjectId
	name := request.Name
	if err := m.pm.Rename(ctx, projectId, userId, name); err != nil {
		return errors.Tag(err, "cannot rename project")
	}
	{
		// Notify real-time
		payload := []interface{}{name}
		if b, err2 := json.Marshal(payload); err2 == nil {
			_ = m.editorEvents.Publish(ctx, &sharedTypes.EditorEventsMessage{
				RoomId:  projectId,
				Message: "projectNameUpdated",
				Payload: b,
			})
		}
	}

	return nil
}
