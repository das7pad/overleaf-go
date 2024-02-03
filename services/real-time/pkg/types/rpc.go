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
	"strconv"
	"sync"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Action string

const (
	JoinDoc           = Action("joinDoc")
	GetConnectedUsers = Action("clientTracking.getConnectedUsers")
	UpdatePosition    = Action("clientTracking.updatePosition")
	ApplyUpdate       = Action("applyUpdate")
	Ping              = Action("ping")
)

type Callback int64

type LazySuccessResponse struct {
	Callback Callback          `json:"c"`
	Latency  sharedTypes.Timed `json:"l"`
}

type RPCRequest struct {
	Action   Action           `json:"a"`
	Body     json.RawMessage  `json:"b"`
	Callback Callback         `json:"c"`
	DocId    sharedTypes.UUID `json:"d"`
}

type RPCResponse struct {
	Body                 json.RawMessage         `json:"b,omitempty"`
	Callback             Callback                `json:"c,omitempty"`
	Error                *errors.JavaScriptError `json:"e,omitempty"`
	Name                 string                  `json:"n,omitempty"`
	Latency              sharedTypes.Timed       `json:"l,omitempty"`
	ProcessedBy          string                  `json:"p,omitempty"`
	LazySuccessResponses []LazySuccessResponse   `json:"s,omitempty"`

	releaseBody *rpcResponseBufEntry
	FatalError  bool `json:"-"`
}

func (r *RPCResponse) ReleaseBuffer() {
	rpcResponseBufPool.Put(r.releaseBody)
}

var rpcResponseBufPool sync.Pool

type rpcResponseBufEntry struct {
	b []byte
}

func getResponseBuffer(n int) *rpcResponseBufEntry {
	if v := rpcResponseBufPool.Get(); v != nil {
		o := v.(*rpcResponseBufEntry)
		if cap(o.b) >= n {
			o.b = o.b[:0]
			return o
		}
	}
	return &rpcResponseBufEntry{b: make([]byte, 0, n)}
}

func (r *RPCResponse) MarshalJSON() ([]byte, error) {
	rb := getResponseBuffer(100 + len(r.Body))
	o := rb.b
	o = append(o, '{')
	c := false
	comma := func() {
		if c {
			o = append(o, ',')
		}
		c = true
	}
	if m := len(r.Body); m > 0 {
		o = append(o, `"b":`...)
		o = append(o, r.Body...)
		c = true
	}
	if r.releaseBody != nil {
		rpcResponseBufPool.Put(r.releaseBody)
		r.releaseBody = nil
	}
	if r.Callback != 0 {
		comma()
		o = append(o, `"c":`...)
		o = strconv.AppendInt(o, int64(r.Callback), 10)
	}
	if r.Error != nil {
		comma()
		o = append(o, `"e":`...)
		blob, err := json.Marshal(r.Error)
		if err != nil {
			rb.b = o
			rpcResponseBufPool.Put(rb)
			return nil, err
		}
		o = append(o, blob...)
	}
	if len(r.Name) > 0 {
		comma()
		o = append(o, `"n":"`...)
		o = append(o, r.Name...)
		o = append(o, '"')
	}
	{
		comma()
		o = append(o, `"l":"`...)
		o = append(o, r.Latency.String()...)
		o = append(o, '"')
	}
	if len(r.ProcessedBy) > 0 {
		comma()
		o = append(o, `"p":"`...)
		o = append(o, r.ProcessedBy...)
		o = append(o, '"')
	}
	if len(r.LazySuccessResponses) > 0 {
		comma()
		o = append(o, `"s":`...)
		blob, err := json.Marshal(r.LazySuccessResponses)
		if err != nil {
			rb.b = o
			rpcResponseBufPool.Put(rb)
			return nil, err
		}
		o = append(o, blob...)
	}
	o = append(o, '}')
	rb.b = o
	r.releaseBody = rb
	return o, nil
}

func (r *RPCResponse) IsLazySuccessResponse() bool {
	if r.Error != nil {
		return false
	}
	if r.Callback > 0 {
		return false
	}
	if len(r.Name) > 0 {
		return false
	}
	return true
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
	if len(r.Request.Body) > sharedTypes.MaxDocSizeBytes {
		return &errors.BodyTooLargeError{}
	}
	return nil
}
