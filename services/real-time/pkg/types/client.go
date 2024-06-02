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
	"encoding/binary"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/das7pad/overleaf-go/pkg/base64Ordered"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/randQueue"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Capabilities uint8

type CapabilityComponent uint8

const (
	NoCapabilities            = Capabilities(0)
	CanEditContent            = CapabilityComponent(2)
	CanSeeOtherClients        = CapabilityComponent(3)
	CanSeeNonRestrictedEvents = CapabilityComponent(5)
	CanSeeAllEditorEvents     = CapabilityComponent(7)
)

func (c Capabilities) Includes(action CapabilityComponent) bool {
	return uint8(c)%uint8(action) == 0
}

func (c Capabilities) CheckIncludes(action CapabilityComponent) error {
	if !c.Includes(action) {
		return &errors.NotAuthorizedError{}
	}
	return nil
}

func (c Capabilities) TakeAway(action CapabilityComponent) Capabilities {
	if !c.Includes(action) {
		return c
	}
	return Capabilities(uint8(c) / uint8(action))
}

func PrepareBulkMessage(response *RPCResponse) (WriteQueueEntry, error) {
	blob, err := response.MarshalJSON()
	if err != nil {
		return WriteQueueEntry{}, err
	}
	pm, err := websocket.NewPreparedMessage(websocket.TextMessage, blob)
	response.ReleaseBuffer()
	if err != nil {
		return WriteQueueEntry{}, err
	}
	return WriteQueueEntry{
		Msg:        pm,
		FatalError: response.FatalError,
	}, nil
}

type WriteQueueEntry struct {
	RPCResponse *RPCResponse
	Msg         *websocket.PreparedMessage
	FatalError  bool
}

type WriteQueue chan WriteQueueEntry

var rng = make(randQueue.Q8, 512)

func init() {
	go rng.Run(2048)
}

const (
	PublicIdLength         = 22
	PublicIdTsPrefixLength = 11
)

// generatePublicId yields a secure unique id
// It contains a timestamp in ns precision and 8 bytes of randomness in b64.
func generatePublicId() sharedTypes.PublicId {
	const publicIdOffset = PublicIdLength - 16
	var buf [PublicIdLength]byte
	binary.BigEndian.PutUint64(
		buf[publicIdOffset:publicIdOffset+8], uint64(time.Now().UnixNano()),
	)
	rnd := <-rng
	copy(buf[publicIdOffset+8:PublicIdLength], rnd[:])
	base64Ordered.Encode(buf[:], buf[publicIdOffset:PublicIdLength])
	return sharedTypes.PublicId(buf[:])
}

func NewClient(conn *websocket.LeanConn, writeQueueDepth int, scheduleWriteQueue chan *Client) *Client {
	c := Client{
		PublicId:           generatePublicId(),
		conn:               conn,
		writeQueue:         make([]WriteQueueEntry, writeQueueDepth),
		scheduleWriteQueue: scheduleWriteQueue,
	}
	c.MarkAsLeftDoc()
	return &c
}

type Clients []*Client

func (c Clients) String() string {
	var s strings.Builder
	s.WriteByte('[')
	for _, client := range c {
		if s.Len() > 1 {
			s.WriteString(", ")
		}
		s.WriteString(client.String())
	}
	s.WriteByte(']')
	return s.String()
}

func (c Clients) Index(needle *Client) int {
	for i, client := range c {
		if client == needle {
			return i
		}
	}
	return -1
}

const (
	pendingWrite              = 1
	enqueuedWrite             = 1 << 8
	tryEnqueueWrite           = enqueuedWrite + pendingWrite
	pendingWritesDisconnected = 1 << 24

	disconnectAfterFlush = 1
	forceDisconnected    = 2
)

type Client struct {
	writeState   atomic.Uint32
	capabilities Capabilities
	HasEmitted   bool // only read after disconnect

	PublicId    sharedTypes.PublicId
	ProjectId   sharedTypes.UUID
	UserId      sharedTypes.UUID
	DisplayName string

	docId atomic.Pointer[sharedTypes.UUID]

	lsr                []LazySuccessResponse
	conn               *websocket.LeanConn
	writeQueue         []WriteQueueEntry
	scheduleWriteQueue chan *Client
}

func (c *Client) String() string {
	return string(c.PublicId)
}

func (c *Client) HasJoinedDoc(id sharedTypes.UUID) bool {
	return id == *c.docId.Load()
}

func (c *Client) MarkAsJoined(id sharedTypes.UUID) {
	c.docId.Store(&id)
}

var docIdNotJoined = &sharedTypes.UUID{}

func (c *Client) MarkAsLeftDoc() {
	c.docId.Store(docIdNotJoined)
}

func (c *Client) ResolveCapabilities(privilegeLevel sharedTypes.PrivilegeLevel, isRestrictedUser project.IsRestrictedUser) {
	switch privilegeLevel {
	case sharedTypes.PrivilegeLevelOwner, sharedTypes.PrivilegeLevelReadAndWrite:
		c.capabilities = Capabilities(
			CanEditContent *
				CanSeeOtherClients *
				CanSeeNonRestrictedEvents *
				CanSeeAllEditorEvents,
		)
	case sharedTypes.PrivilegeLevelReadOnly:
		c.capabilities = Capabilities(
			CanSeeOtherClients *
				CanSeeNonRestrictedEvents *
				CanSeeAllEditorEvents,
		)
	default:
		c.capabilities = NoCapabilities
	}
	if isRestrictedUser {
		c.capabilities = c.capabilities.TakeAway(CanSeeOtherClients)
		c.capabilities = c.capabilities.TakeAway(CanSeeAllEditorEvents)
	}
}

func (c *Client) HasCapability(component CapabilityComponent) bool {
	return c.capabilities.Includes(component)
}

func (c *Client) CheckHasCapability(component CapabilityComponent) error {
	return c.capabilities.CheckIncludes(component)
}

func (c *Client) CanDo(action Action, docId sharedTypes.UUID) error {
	switch action {
	case Ping:
		return nil
	case JoinDoc:
		return nil
	case ApplyUpdate:
		if !c.HasJoinedDoc(docId) {
			return &errors.InvalidStateError{Msg: "join doc first"}
		}
		if err := c.CheckHasCapability(CanEditContent); err != nil {
			return err
		}
		return nil
	case GetConnectedUsers:
		if err := c.CheckHasCapability(CanSeeOtherClients); err != nil {
			return err
		}
		return nil
	case UpdatePosition:
		if err := c.CheckHasCapability(CanSeeOtherClients); err != nil {
			return err
		}
		return nil
	default:
		return &errors.ValidationError{
			Msg: "unknown action: " + string(action),
		}
	}
}

func (c *Client) ForceDisconnect() {
	if c.initiateDisconnect(forceDisconnected) {
		c.scheduleWriteQueue <- c
	}
}

func (c *Client) TriggerDisconnect() {
	if c.initiateDisconnect(disconnectAfterFlush) {
		c.scheduleWriteQueue <- c
	}
}

func (c *Client) initiateDisconnect(v uint32) bool {
	for {
		s := c.writeState.Load()
		closing, r, w := s>>24, uint8(s>>16), uint8(s>>8)
		if closing >= v {
			return false
		}
		ok := c.writeState.CompareAndSwap(s, s|(v-closing)*pendingWritesDisconnected)
		if !ok {
			continue
		}

		return r == w
	}
}

func (c *Client) ProcessQueuedMessages() {
	for {
		hasMore, ok := c.processNextQueuedMessage()
		if !ok {
			c.initiateDisconnect(forceDisconnected)
			_ = c.conn.Close()
			return
		}
		if !hasMore {
			return
		}
	}
}

func (c *Client) processNextQueuedMessage() (bool, bool) {
	s := c.writeState.Load()
	closing, r, w, pending := s>>24, uint8(s>>16), uint8(s>>8), uint8(s)
	if closing >= forceDisconnected {
		return false, false
	}
	if pending > 0 {
		return true, true
	}
	if r == w {
		return false, closing == 0
	}
	n := uint8(cap(c.writeQueue))
	r = (r + 1) % n
	entry := c.writeQueue[r]
	c.writeQueue[r] = WriteQueueEntry{}
	if entry.Msg != nil {
		if err := c.conn.WritePreparedMessage(entry.Msg); err != nil {
			return false, false
		}
	} else if len(c.lsr) < 15 && entry.RPCResponse.IsLazySuccessResponse() {
		c.lsr = append(c.lsr, LazySuccessResponse{
			Callback: entry.RPCResponse.Callback,
			Latency:  entry.RPCResponse.Latency,
		})
	} else {
		if ok := c.writeResponse(*entry.RPCResponse); !ok {
			return false, false
		}
	}
	for {
		w = w % n
		sRolled := closing<<24 | uint32(r)<<16 | uint32(w)<<8 | uint32(pending)
		if c.writeState.CompareAndSwap(s, sRolled) {
			break
		}
		s = c.writeState.Load()
		closing, r, w, pending = s>>24, uint8(s>>16), uint8(s>>8), uint8(s)
		r = (r + 1) % n
	}
	if r != w {
		return true, !entry.FatalError
	}
	return false, closing == 0
}

func (c *Client) writeResponse(response RPCResponse) bool {
	if len(c.lsr) > 0 {
		response.LazySuccessResponses = c.lsr
		c.lsr = c.lsr[:0]
	}
	blob, err := response.MarshalJSON()
	if err != nil {
		return false
	}
	err = c.conn.WriteMessage(websocket.TextMessage, blob)
	response.ReleaseBuffer()
	if err != nil {
		return false
	}
	return true
}

func (c *Client) TryWriteResponseOrQueue(response RPCResponse) bool {
	s := c.writeState.Add(tryEnqueueWrite)
	closing, r, w, pending := s>>24, uint8(s>>16), uint8(s>>8), uint8(s)
	if closing > 0 {
		c.writeState.Add(^uint32(tryEnqueueWrite - 1))
		return false
	}
	if r == w-1 && pending == 1 {
		ok := c.writeResponse(response)
		c.writeState.Add(^uint32(tryEnqueueWrite - 1))
		return ok
	}
	c.writeState.Add(^uint32(tryEnqueueWrite - 1))
	return c.EnsureQueueResponse(response)
}

func (c *Client) EnsureQueueResponse(response RPCResponse) bool {
	return c.EnsureQueueMessage(WriteQueueEntry{
		RPCResponse: &response,
		FatalError:  response.FatalError,
	})
}

func (c *Client) EnsureQueueMessage(msg WriteQueueEntry) bool {
	s := c.writeState.Add(tryEnqueueWrite)
	closing, r, w, pending := s>>24, uint8(s>>16), uint8(s>>8), uint8(s)
	if closing > 0 {
		// The client is in the process of disconnecting.
		c.writeState.Add(^uint32(tryEnqueueWrite - 1))
		return false
	}
	idx := int(w) % cap(c.writeQueue)
	if r == w-pending+1 {
		// The queue is full, we need to drop this message.
		c.writeState.Add(^uint32(tryEnqueueWrite - 1))
		// In dropping this message, the client went out of sync, disconnect.
		c.ForceDisconnect()
		return false
	}
	c.writeQueue[idx] = msg
	c.writeState.Add(^uint32(pendingWrite - 1))
	if r == w-1 {
		c.scheduleWriteQueue <- c
	}
	return true
}
