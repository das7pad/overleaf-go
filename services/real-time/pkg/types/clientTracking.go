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

func appendDisplayName(p []byte, name string, comma bool) []byte {
	if len(name) == 0 {
		return p
	}
	ok := true
	for i := 0; ok && i < len(name); i++ {
		v := name[i]
		if v == '"' || v == '\\' || v < 32 || v > 126 {
			ok = false
		}
	}
	if ok {
		if comma {
			p = append(p, ',')
		}
		p = append(p, `"n":"`...)
		p = append(p, name...)
		p = append(p, '"')
	} else {
		blob, err := json.Marshal(name)
		if err == nil {
			if comma {
				p = append(p, ',')
			}
			p = append(p, `"n":`...)
			p = append(p, blob...)
		}
	}
	return p
}

type ConnectingConnectedClient struct {
	DisplayName string `json:"n,omitempty"`
}

func (c ConnectingConnectedClient) Append(p []byte) []byte {
	p = append(p, '{')
	p = appendDisplayName(p, c.DisplayName, false)
	p = append(p, '}')
	return p
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
	IsJoin      uint8                `json:"j,omitempty"`
	HasEmitted  bool                 `json:"-"`
}

func (r RoomChange) Append(p []byte) []byte {
	p = append(p, `{"i":"`...)
	p = append(p, r.PublicId...)
	p = append(p, '"')
	if r.IsJoin != 0 {
		p = appendDisplayName(p, r.DisplayName, true)
		p = append(p, `,"j":1`...)
	}
	p = append(p, '}')
	return p
}

type RoomChanges []RoomChange
