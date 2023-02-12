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

package types

import (
	"encoding/json"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Action string

const (
	JoinDoc           = Action("joinDoc")
	LeaveDoc          = Action("leaveDoc")
	GetConnectedUsers = Action("clientTracking.getConnectedUsers")
	UpdatePosition    = Action("clientTracking.updatePosition")
	ApplyUpdate       = Action("applyUpdate")
	Ping              = Action("ping")
)

type Callback int64

type RPCRequest struct {
	Action   Action           `json:"a"`
	Body     json.RawMessage  `json:"b"`
	Callback Callback         `json:"c"`
	DocId    sharedTypes.UUID `json:"d"`
}

type RPCResponse struct {
	Body        json.RawMessage         `json:"b,omitempty"`
	Callback    Callback                `json:"c,omitempty"`
	Error       *errors.JavaScriptError `json:"e,omitempty"`
	Name        string                  `json:"n,omitempty"`
	Latency     sharedTypes.Timed       `json:"l,omitempty"`
	ProcessedBy string                  `json:"p,omitempty"`

	FatalError bool `json:"-"`
}

type RPC struct {
	Client   *Client
	Request  *RPCRequest
	Response *RPCResponse
}

func (r *RPC) Validate() error {
	if r.Client == nil {
		return &errors.ValidationError{Msg: "missing rpc detail: client"}
	}
	if r.Request == nil {
		return &errors.ValidationError{Msg: "missing rpc detail: request"}
	}
	if r.Response == nil {
		return &errors.ValidationError{Msg: "missing rpc detail: response"}
	}
	if err := r.Client.CanDo(r.Request.Action, r.Request.DocId); err != nil {
		return err
	}
	return nil
}
