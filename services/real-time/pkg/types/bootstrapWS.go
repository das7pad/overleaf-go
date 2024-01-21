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

	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type BootstrapWSResponse struct {
	// Project contains (cached) serialized types.ProjectDetails
	Project        json.RawMessage            `json:"project"`
	PrivilegeLevel sharedTypes.PrivilegeLevel `json:"privilegeLevel"`
	// ConnectedClients contains (shared) serialized types.ConnectedClients
	ConnectedClients json.RawMessage      `json:"connectedClients"`
	PublicId         sharedTypes.PublicId `json:"publicId"`
}

func (b *BootstrapWSResponse) WriteInto(resp *RPCResponse) {
	o := getResponseBuffer(100 + len(`{"project":,"privilegeLevel":"","connectedClients":,"publicId":""}`) + len(b.Project) + len(b.PrivilegeLevel) + len(b.ConnectedClients) + len(b.PublicId))
	o = append(o, `{"project":`...)
	o = append(o, b.Project...)
	o = append(o, `,"privilegeLevel":"`...)
	o = append(o, b.PrivilegeLevel...)
	if b.ConnectedClients == nil {
		o = append(o, '"')
	} else {
		o = append(o, `","connectedClients":`...)
		o = append(o, b.ConnectedClients...)
	}
	o = append(o, `,"publicId":"`...)
	o = append(o, b.PublicId...)
	o = append(o, `"}`...)
	resp.Body = o
	resp.releaseBody = true
}

type ProjectDetails struct {
	project.ForBootstrapWS
	project.OwnerFeaturesField
	project.TokensField                   // stub
	RootFolder          []*project.Folder `json:"rootFolder"`
}
