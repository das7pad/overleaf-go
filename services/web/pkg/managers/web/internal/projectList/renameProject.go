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

package projectList

import (
	"context"
	"encoding/json"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func (m *manager) RenameProject(ctx context.Context, request *types.RenameProjectRequest) error {
	if err := request.Name.Validate(); err != nil {
		return err
	}
	userId := request.Session.User.Id
	projectId := request.ProjectId
	name := request.Name
	if err := m.pm.Rename(ctx, projectId, userId, name); err != nil {
		return errors.Tag(err, "rename project")
	}

	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	if blob, err := json.Marshal(name); err == nil {
		_ = m.editorEvents.Publish(ctx, &sharedTypes.EditorEventsMessage{
			RoomId:  projectId,
			Message: "projectNameUpdated",
			Payload: blob,
		})
	}

	return nil
}
