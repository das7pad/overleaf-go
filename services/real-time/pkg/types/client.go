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
	"math"
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

func NewClient(writeQueue WriteQueue) *Client {
	c := Client{
		PublicId:   generatePublicId(),
		writeQueue: writeQueue,
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

const pendingWritesDisconnected = math.MinInt32

type Client struct {
	capabilities  Capabilities
	pendingWrites atomic.Int32

	PublicId    sharedTypes.PublicId
	ProjectId   sharedTypes.UUID
	UserId      sharedTypes.UUID
	DisplayName string

	docId atomic.Pointer[sharedTypes.UUID]

	writeQueue WriteQueue
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

func (c *Client) TriggerDisconnectAndDropQueue() {
	c.TriggerDisconnect()
	for range c.writeQueue {
	}
}

func (c *Client) TriggerDisconnect() {
	for !c.pendingWrites.CompareAndSwap(0, pendingWritesDisconnected) {
		if c.pendingWrites.Load() < 0 {
			return
		}
		time.Sleep(time.Nanosecond)
	}
	close(c.writeQueue)
}

func (c *Client) EnsureQueueResponse(response *RPCResponse) bool {
	return c.EnsureQueueMessage(WriteQueueEntry{
		RPCResponse: response,
		FatalError:  response.FatalError,
	})
}

func (c *Client) EnsureQueueMessage(msg WriteQueueEntry) bool {
	if c.pendingWrites.Add(1) < 0 {
		// The client is in the process of disconnecting
		c.pendingWrites.Add(-1)
		return false
	}
	select {
	case c.writeQueue <- msg:
		c.pendingWrites.Add(-1)
		return true
	default:
		// The queue is full, dropping message
		c.pendingWrites.Add(-1)
		// The client is out of sync, disconnect and flush queue
		c.TriggerDisconnectAndDropQueue()
		return false
	}
}
