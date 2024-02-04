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
	"bytes"
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
	/* "h" is a virtual field indicating the length of Body */
	Body                 json.RawMessage                `json:"b,omitempty"`
	Callback             Callback                       `json:"c,omitempty"`
	Error                *errors.JavaScriptError        `json:"e,omitempty"`
	Name                 sharedTypes.EditorEventMessage `json:"n,omitempty"`
	Latency              sharedTypes.Timed              `json:"l,omitempty"`
	ProcessedBy          string                         `json:"p,omitempty"`
	LazySuccessResponses []LazySuccessResponse          `json:"s,omitempty"`

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
	o = append(o, `"h":`...)
	bodyHint := uint64(len(r.Body))
	o = strconv.AppendUint(o, bodyHint, 10)
	if bodyHint > 0 {
		o = append(o, `,"b":`...)
		o = append(o, r.Body...)
	}
	if r.releaseBody != nil {
		rpcResponseBufPool.Put(r.releaseBody)
		r.releaseBody = nil
	}
	if r.Callback != 0 {
		o = append(o, `,"c":`...)
		o = strconv.AppendInt(o, int64(r.Callback), 10)
	}
	if r.Error != nil {
		o = append(o, `,"e":`...)
		blob, err := json.Marshal(r.Error)
		if err != nil {
			rb.b = o
			rpcResponseBufPool.Put(rb)
			return nil, err
		}
		o = append(o, blob...)
	}
	if len(r.Name) > 0 {
		o = append(o, `,"n":"`...)
		o = append(o, r.Name...)
		o = append(o, '"')
	}
	{
		o = append(o, `,"l":"`...)
		o = append(o, r.Latency.String()...)
		o = append(o, '"')
	}
	if len(r.ProcessedBy) > 0 {
		o = append(o, `,"p":"`...)
		o = append(o, r.ProcessedBy...)
		o = append(o, '"')
	}
	if len(r.LazySuccessResponses) > 0 {
		o = append(o, `,"s":`...)
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

var errMissingBodyHint = errors.New("bad RPCResponse: missing body hint")

var (
	rpcResponseBodyHint = []byte(`{"h":`)
	rpcResponseBody     = []byte(`"b":`)
)

func (r *RPCResponse) parseBodyHint(p []byte) (int, int) {
	if !bytes.HasPrefix(p, rpcResponseBodyHint) || len(p) < 7 {
		return -1, 0
	}
	if p[5] == '0' && p[6] == ',' {
		return 7, 0
	}
	idx := bytes.IndexByte(p[5:], ',')
	if idx == -1 || idx == 0 || !bytes.HasPrefix(p[5+idx+1:], rpcResponseBody) {
		return -1, 0
	}
	idx += 5
	v, err := strconv.ParseUint(string(p[5:idx]), 10, 64)
	if err != nil {
		return -1, 0
	}
	h := int(v)
	idx += 1 + len(rpcResponseBody)
	if len(p) < idx+h+1 {
		return -1, 0
	}
	return idx, h
}

var errBadRPCResponse = errors.New("bad RPCResponse")

func (r *RPCResponse) FastUnmarshalJSON(p []byte) error {
	if len(p) < 2 || p[0] != '{' || p[len(p)-1] != '}' {
		return errBadRPCResponse
	}
	p[len(p)-1] = ','
	i, next := r.parseBodyHint(p)
	if i == -1 {
		return errMissingBodyHint
	}
	if next > 0 {
		j := i + next
		r.Body = p[i:j]
		i = j + 1
	}
	if next = bytes.IndexByte(p[i:], ','); next == -1 {
		return errBadRPCResponse
	}
	j := i + next
	if bytes.HasPrefix(p[i:], []byte(`"c":`)) {
		v, err := strconv.ParseInt(string(p[i+4:j]), 10, 64)
		if err != nil {
			return errBadRPCResponse
		}
		r.Callback = Callback(v)
		i = j + 1
		if next = bytes.IndexByte(p[i:], ','); next == -1 {
			return errBadRPCResponse
		}
		j = i + next
	}
	if bytes.HasPrefix(p[i:], []byte(`"e":`)) {
		r.Error = &errors.JavaScriptError{}
		j = i + bytes.IndexByte(p[i:], '}')
		if err := json.Unmarshal(p[i+4:j], r.Error); err != nil {
			return errBadRPCResponse
		}
		i = j + 1
		if next = bytes.IndexByte(p[i:], ','); next == -1 {
			return errBadRPCResponse
		}
		j = i + next
	}
	if bytes.HasPrefix(p[i:], []byte(`"n":`)) {
		if err := r.Name.UnmarshalJSON(p[i+4 : j]); err != nil {
			return errBadRPCResponse
		}
		i = j + 1
		if next = bytes.IndexByte(p[i:], ','); next == -1 {
			return errBadRPCResponse
		}
		j = i + next
	}
	if bytes.HasPrefix(p[i:], []byte(`"l":`)) {
		if err := r.Latency.UnmarshalJSON(p[i+4 : j]); err != nil {
			return errBadRPCResponse
		}
		i = j + 1
		if next = bytes.IndexByte(p[i:], ','); next == -1 {
			j = len(p)
		} else {
			j = i + next
		}
	}
	if bytes.HasPrefix(p[i:], []byte(`"p":`)) {
		if p[i+4] != '"' || p[j-1] != '"' {
			return errBadRPCResponse
		}
		r.ProcessedBy = string(p[i+5 : j-1])
		i = j + 1
	}
	if bytes.HasPrefix(p[i:], []byte(`"s":`)) {
		if p[i+4] != '[' || p[len(p)-2] != ']' {
			return errBadRPCResponse
		}
		i += 5
		n := bytes.Count(p[i:], []byte("c"))
		lsr := make([]LazySuccessResponse, n)
		for l := 0; l < n; l++ {
			if l > 0 {
				if p[i] != ',' {
					return errBadRPCResponse
				}
				i++
			}
			if !bytes.HasPrefix(p[i:], []byte(`{"c":`)) {
				return errBadRPCResponse
			}
			i += 5
			if next = bytes.IndexByte(p[i:], ','); next == -1 {
				return errBadRPCResponse
			}
			j = i + next
			if !bytes.HasPrefix(p[j+1:], []byte(`"l":"`)) {
				return errBadRPCResponse
			}
			v, err := strconv.ParseInt(string(p[i:j]), 10, 64)
			if err != nil {
				return errBadRPCResponse
			}
			lsr[l].Callback = Callback(v)
			i = j + 1 + 4
			j = i + bytes.IndexByte(p[i:], '}')
			if err = lsr[l].Latency.UnmarshalJSON(p[i:j]); err != nil {
				return errBadRPCResponse
			}
			i = j + 1
		}
		r.LazySuccessResponses = lsr
	}
	return nil
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
