// Golang port of Overleaf
// Copyright (C) 2021-2024 Jakob Ackermann <das7pad@outlook.com>
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

package types

import (
	"encoding/json"

	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type ClientPosition struct {
	Row      int64            `json:"r,omitempty"`
	Column   int64            `json:"c,omitempty"`
	EntityId sharedTypes.UUID `json:"e"`
}

type ConnectedClient struct {
	ClientId    sharedTypes.PublicId `json:"i,omitempty"`
	DisplayName string               `json:"n,omitempty"`
	ClientPosition
}

type ConnectingConnectedClient struct {
	DisplayName string `json:"n,omitempty"`
}

type ConnectedClients []ConnectedClient

type GetConnectedUsersResponse struct {
	// ConnectedClients contains (shared) serialized types.ConnectedClients
	ConnectedClients json.RawMessage `json:"connectedClients"`
}

func (g *GetConnectedUsersResponse) MarshalJSON() ([]byte, error) {
	o := make([]byte, 0, len(`{"connectedClients":}`)+len(g.ConnectedClients))
	o = append(o, `{"connectedClients":`...)
	o = append(o, g.ConnectedClients...)
	o = append(o, '}')
	return o, nil
}

type RoomChange struct {
	PublicId    sharedTypes.PublicId `json:"i"`
	DisplayName string               `json:"n,omitempty"`
	IsJoin      bool                 `json:"j,omitempty"`
}

type RoomChanges []RoomChange
