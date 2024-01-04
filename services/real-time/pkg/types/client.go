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
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
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
	blob, err := json.Marshal(response)
	if err != nil {
		return WriteQueueEntry{}, err
	}
	pm, err := websocket.NewPreparedMessage(websocket.TextMessage, blob)
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

type WriteQueue chan<- WriteQueueEntry

// generatePublicId yields a secure unique id
// It contains a timestamp in ns precision and 8 bytes of randomness in b64.
func generatePublicId() (sharedTypes.PublicId, error) {
	const (
		// publicIdLength = base64.RawURLEncoding.EncodedLen(16)
		publicIdLength = 22
		publicIdOffset = publicIdLength - 16
	)
	buf := make([]byte, publicIdLength)
	if _, err := rand.Read(buf[publicIdOffset+8:]); err != nil {
		return "", err
	}
	binary.BigEndian.AppendUint64(
		buf[publicIdOffset:publicIdOffset], uint64(time.Now().UnixNano()),
	)
	base64.RawURLEncoding.Encode(buf, buf[publicIdOffset:])
	return sharedTypes.PublicId(buf[0:publicIdLength]), nil
}

func NewClient(writeQueue WriteQueue, disconnect func()) (*Client, error) {
	publicId, err := generatePublicId()
	if err != nil {
		return nil, err
	}
	c := Client{
		PublicId:   publicId,
		writeQueue: writeQueue,
		disconnect: disconnect,
	}
	c.MarkAsLeftDoc()
	return &c, nil
}

type Clients []*Client

func (c Clients) Index(needle *Client) int {
	for i, client := range c {
		if client == needle {
			return i
		}
	}
	return -1
}

type Client struct {
	capabilities Capabilities

	PublicId    sharedTypes.PublicId
	ProjectId   sharedTypes.UUID
	UserId      sharedTypes.UUID
	DisplayName string

	docId atomic.Pointer[sharedTypes.UUID]

	writeQueue WriteQueue
	disconnect func()
}

func (c *Client) String() string {
	return string(c.PublicId)
}

func (c *Client) CloseWriteQueue() {
	close(c.writeQueue)
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

func (c *Client) TriggerDisconnect() {
	c.disconnect()
}

func (c *Client) QueueResponse(response *RPCResponse) error {
	return c.QueueMessage(WriteQueueEntry{
		RPCResponse: response,
		FatalError:  response.FatalError,
	})
}

func (c *Client) EnsureQueueResponse(response *RPCResponse) bool {
	if err := c.QueueResponse(response); err != nil {
		// Client is out-of-sync.
		c.TriggerDisconnect()
		return false
	}
	return true
}

func (c *Client) QueueMessage(msg WriteQueueEntry) error {
	select {
	case c.writeQueue <- msg:
		return nil
	default:
		return errors.New("queue is full")
	}
}

func (c *Client) EnsureQueueMessage(msg WriteQueueEntry) bool {
	if err := c.QueueMessage(msg); err != nil {
		// Client is out-of-sync.
		c.TriggerDisconnect()
		return false
	}
	return true
}
