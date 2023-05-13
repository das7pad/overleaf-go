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

package editorEvents

import (
	"time"

	"github.com/das7pad/overleaf-go/pkg/pubSub/channel"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/managers/realTime/internal/broadcaster"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/managers/realTime/internal/clientTracking"
)

type Manager interface {
	broadcaster.Broadcaster
}

func New(c channel.Manager, clientTracking clientTracking.Manager) Manager {
	newRoom := func(room *broadcaster.TrackingRoom) broadcaster.Room {
		now := time.Now()
		return &ProjectRoom{
			TrackingRoom:       room,
			clientTracking:     clientTracking,
			nextProjectRefresh: now,
			nextClientRefresh:  now,
		}
	}
	b := broadcaster.New(c, newRoom)
	return &manager{
		Broadcaster: b,
	}
}

type manager struct {
	broadcaster.Broadcaster
}
