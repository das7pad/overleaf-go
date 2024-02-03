// Golang port of Overleaf
// Copyright (C) 2023-2024 Jakob Ackermann <das7pad@outlook.com>
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

package clientTracking

import (
	"context"
	"encoding/json"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

func (m *manager) notifyUpdated(ctx context.Context, client *types.Client, p types.ClientPosition) error {
	body, err := json.Marshal(types.ConnectedClient{
		ClientId:       client.PublicId,
		ClientPosition: p,
	})
	if err != nil {
		return errors.Tag(err, "encode notification")
	}
	msg := sharedTypes.EditorEvent{
		Source:  client.PublicId,
		RoomId:  client.ProjectId,
		Message: sharedTypes.ClientTrackingUpdated,
		Payload: body,
	}
	if err = m.c.Publish(ctx, &msg); err != nil {
		return errors.Tag(err, "send notification for client updated")
	}
	return nil
}
