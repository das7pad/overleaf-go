// Golang port of the Overleaf real-time service
// Copyright (C) 2021 Jakob Ackermann <das7pad@outlook.com>
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
	"encoding/hex"
	"encoding/json"
	"math/rand"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Capabilities int
type CapabilityComponent int

const (
	NoCapabilities            = Capabilities(0)
	CanAddComment             = CapabilityComponent(2)
	CanEditContent            = CapabilityComponent(3)
	CanSeeOtherClients        = CapabilityComponent(5)
	CanSeeComments            = CapabilityComponent(7)
	CanSeeNonRestrictedEvents = CapabilityComponent(11)
	CanSeeAllEditorEvents     = CapabilityComponent(13)
)

func (c Capabilities) Includes(action CapabilityComponent) bool {
	return int(c)%int(action) == 0
}

func (c Capabilities) CheckIncludes(action CapabilityComponent) error {
	if !c.Includes(action) {
		return &errors.NotAuthorizedError{}
	}
	return nil
}

func (c Capabilities) TakeAway(action CapabilityComponent) Capabilities {
	if err := c.CheckIncludes(action); err != nil {
		return c
	}
	return Capabilities(int(c) / int(action))
}

func PrepareBulkMessage(response *RPCResponse) (*WriteQueueEntry, error) {
	blob, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}
	pm, err := websocket.NewPreparedMessage(websocket.TextMessage, blob)
	if err != nil {
		return nil, err
	}
	return &WriteQueueEntry{
		Blob:       blob,
		Msg:        pm,
		FatalError: response.FatalError,
	}, nil
}

type WriteQueueEntry struct {
	Blob       []byte
	Msg        *websocket.PreparedMessage
	FatalError bool
}

type WriteQueue chan<- *WriteQueueEntry

// generatePublicId yields a secure unique id
// It contains a 16 hex char long timestamp in ns precision, a hyphen and
//  another 16 hex char long random string.
func generatePublicId() (sharedTypes.PublicId, error) {
	buf := make([]byte, 8)
	_, err := rand.Read(buf)
	if err != nil {
		return "", err
	}
	now := time.Now().UnixNano()
	id := sharedTypes.PublicId(
		strconv.FormatInt(now, 16) + "-" + hex.EncodeToString(buf),
	)
	return id, nil
}

func NewClient(wsBootstrap *WsBootstrap, writerChanges chan bool, writeQueue WriteQueue, disconnect func()) (*Client, error) {
	publicId, err := generatePublicId()
	if err != nil {
		return nil, err
	}
	return &Client{
		lockedProjectId: wsBootstrap.ProjectId,
		PublicId:        publicId,
		User:            wsBootstrap.User,
		writerChanges:   writerChanges,
		writeQueue:      writeQueue,
		disconnect:      disconnect,
	}, nil
}

type Client struct {
	capabilities    Capabilities
	lockedProjectId primitive.ObjectID

	DocId     *primitive.ObjectID
	PublicId  sharedTypes.PublicId
	ProjectId *primitive.ObjectID
	User      *User

	knownDocs []primitive.ObjectID

	writerChanges chan bool
	writeQueue    WriteQueue
	disconnect    func()
}

func (c *Client) AddWriter() {
	c.writerChanges <- true
}

func (c *Client) RemoveWriter() {
	c.writerChanges <- false
}

func (c *Client) IsKnownDoc(id primitive.ObjectID) bool {
	if c.knownDocs == nil {
		return false
	}
	for _, doc := range c.knownDocs {
		if doc == id {
			return true
		}
	}
	return false
}

const MaxKnownDocsToKeep = 100

func (c *Client) AddKnownDoc(id primitive.ObjectID) {
	if len(c.knownDocs) < MaxKnownDocsToKeep {
		c.knownDocs = append(c.knownDocs, id)
	} else {
		c.knownDocs[rand.Int63n(MaxKnownDocsToKeep)] = id
	}
}

type PrivilegeLevel string
type IsRestrictedUser bool

func (c *Client) ResolveCapabilities(privilegeLevel PrivilegeLevel, isRestrictedUser IsRestrictedUser) {
	switch privilegeLevel {
	case "owner", "readAndWrite":
		c.capabilities = Capabilities(
			CanAddComment *
				CanEditContent *
				CanSeeOtherClients *
				CanSeeComments *
				CanSeeNonRestrictedEvents *
				CanSeeAllEditorEvents,
		)
	case "readOnly":
		c.capabilities = Capabilities(
			CanAddComment *
				CanSeeOtherClients *
				CanSeeComments *
				CanSeeNonRestrictedEvents *
				CanSeeAllEditorEvents,
		)
	default:
		c.capabilities = NoCapabilities
	}
	if isRestrictedUser {
		c.capabilities = c.capabilities.TakeAway(CanSeeOtherClients)
		c.capabilities = c.capabilities.TakeAway(CanSeeComments)
		c.capabilities = c.capabilities.TakeAway(CanSeeAllEditorEvents)
	}
}

func (c *Client) requireJoinedProject() error {
	if c.ProjectId == nil {
		return &errors.InvalidStateError{Msg: "join project first"}
	}
	return nil
}

func (c *Client) requireJoinedProjectAndDoc() error {
	if err := c.requireJoinedProject(); err != nil {
		return err
	}
	if c.DocId == nil {
		return &errors.InvalidStateError{Msg: "join doc first"}
	}
	return nil
}

func (c *Client) CanJoinProject(id primitive.ObjectID) error {
	if id.Hex() != c.lockedProjectId.Hex() {
		return &errors.NotAuthorizedError{}
	}
	return nil
}

func (c *Client) HasCapability(component CapabilityComponent) bool {
	return c.capabilities.Includes(component)
}

func (c *Client) CheckHasCapability(component CapabilityComponent) error {
	return c.capabilities.CheckIncludes(component)
}

func (c *Client) CanDo(action Action, docId primitive.ObjectID) error {
	switch action {
	case Ping:
		return nil
	case JoinProject:
		if c.ProjectId != nil {
			return &errors.InvalidStateError{Msg: "already joined project"}
		}
		return nil
	case JoinDoc:
		if err := c.requireJoinedProject(); err != nil {
			return err
		}
		if c.DocId != nil && !bytes.Equal(c.DocId[:], docId[:]) {
			return &errors.InvalidStateError{Msg: "leave other doc first"}
		}
		return nil
	case LeaveDoc:
		if err := c.requireJoinedProject(); err != nil {
			return err
		}
		if c.DocId == nil {
			// Silently ignore not joined yet.
			return nil
		}
		return nil
	case ApplyUpdate:
		if err := c.requireJoinedProjectAndDoc(); err != nil {
			return err
		}
		if err := c.capabilities.CheckIncludes(CanEditContent); err != nil {
			return err
		}
		return nil

	case AddComment:
		if err := c.requireJoinedProjectAndDoc(); err != nil {
			return err
		}
		if err := c.capabilities.CheckIncludes(CanAddComment); err != nil {
			return err
		}
		return nil
	case GetConnectedUsers:
		if err := c.requireJoinedProject(); err != nil {
			return err
		}
		if err := c.capabilities.CheckIncludes(CanSeeOtherClients); err != nil {
			return err
		}
		return nil
	case UpdatePosition:
		if err := c.requireJoinedProject(); err != nil {
			return err
		}
		if c.DocId == nil || docId != *c.DocId {
			return &errors.ValidationError{Msg: "stale position update"}
		}
		if err := c.capabilities.CheckIncludes(CanSeeOtherClients); err != nil {
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
	blob, err := json.Marshal(response)
	if err != nil {
		return err
	}
	return c.QueueMessage(&WriteQueueEntry{
		Blob:       blob,
		FatalError: response.FatalError,
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

func (c *Client) QueueMessage(msg *WriteQueueEntry) error {
	select {
	case c.writeQueue <- msg:
		return nil
	default:
		return errors.New("queue is full")
	}
}

func (c *Client) EnsureQueueMessage(msg *WriteQueueEntry) bool {
	if err := c.QueueMessage(msg); err != nil {
		// Client is out-of-sync.
		c.TriggerDisconnect()
		return false
	}
	return true
}
