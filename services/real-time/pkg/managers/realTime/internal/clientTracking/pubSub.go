// Golang port of Overleaf
// Copyright (C) 2023 Jakob Ackermann <das7pad@outlook.com>
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

const (
	ClientConnected    = "clientTracking.clientConnected"
	ClientDisconnected = "clientTracking.clientDisconnected"
	ClientUpdated      = "clientTracking.clientUpdated"
)

func (m *manager) notifyDisconnected(ctx context.Context, client *types.Client) error {
	body, err := json.Marshal(client.PublicId)
	if err != nil {
		return errors.Tag(err, "serialize public id")
	}
	msg := &sharedTypes.EditorEventsMessage{
		RoomId:  client.ProjectId,
		Message: ClientDisconnected,
		Payload: body,
	}
	if err = m.c.Publish(ctx, msg); err != nil {
		return errors.Tag(err, "send notification for client disconnect")
	}
	return nil
}

func (m *manager) notifyUpdated(ctx context.Context, client *types.Client, p types.ClientPosition) error {
	body, err := json.Marshal(types.ClientPositionUpdateNotification{
		Source:         client.PublicId,
		ClientPosition: p,
	})
	if err != nil {
		return errors.Tag(err, "encode notification")
	}
	msg := &sharedTypes.EditorEventsMessage{
		Source:  client.PublicId,
		RoomId:  client.ProjectId,
		Message: ClientUpdated,
		Payload: body,
	}
	if err = m.c.Publish(ctx, msg); err != nil {
		return errors.Tag(err, "send notification for client updated")
	}
	return nil
}

func (m *manager) notifyConnected(ctx context.Context, client *types.Client) error {
	body, err := json.Marshal(types.ConnectedClient{
		ClientId:    client.PublicId,
		DisplayName: client.DisplayName,
	})
	if err != nil {
		return errors.Tag(err, "serialize connected client")
	}
	msg := &sharedTypes.EditorEventsMessage{
		Source:  client.PublicId,
		RoomId:  client.ProjectId,
		Message: ClientConnected,
		Payload: body,
	}
	if err = m.c.Publish(ctx, msg); err != nil {
		return errors.Tag(err, "send notification for client connected")
	}
	return nil
}
