// Golang port of the Overleaf real-time service
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

package editorEvents

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/das7pad/real-time/pkg/managers/realTime/internal/broadcaster"
	"github.com/das7pad/real-time/pkg/managers/realTime/internal/channel"
	"github.com/das7pad/real-time/pkg/managers/realTime/internal/clientTracking"
	"github.com/das7pad/real-time/pkg/types"
)

type Manager interface {
	broadcaster.Broadcaster

	Broadcast(ctx context.Context, message *types.EditorEventsMessage) error
}

func New(ctx context.Context, client redis.UniversalClient, clientTracking clientTracking.Manager) (Manager, error) {
	c, err := channel.New(ctx, client, "editor-events")
	if err != nil {
		return nil, err
	}
	newRoom := func(room *broadcaster.TrackingRoom) broadcaster.Room {
		now := time.Now()
		return &ProjectRoom{
			TrackingRoom:       room,
			clientTracking:     clientTracking,
			nextProjectRefresh: now,
			nextClientRefresh:  now,
		}
	}
	b := broadcaster.New(ctx, c, newRoom)
	return &manager{
		Broadcaster: b,
		c:           c,
	}, nil
}

type manager struct {
	broadcaster.Broadcaster

	c channel.Manager
}

func (m *manager) Broadcast(ctx context.Context, message *types.EditorEventsMessage) error {
	body, err := json.Marshal(message)
	if err != nil {
		return err
	}
	return m.c.Publish(ctx, message.RoomId, string(body))
}
