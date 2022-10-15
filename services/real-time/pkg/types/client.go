// Golang port of Overleaf
// Copyright (C) 2021-2022 Jakob Ackermann <das7pad@outlook.com>
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
	secureRand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	weakRand "math/rand"
	"strconv"
	"time"

	"github.com/gorilla/websocket"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Capabilities int

type CapabilityComponent int

const (
	NoCapabilities            = Capabilities(0)
	CanAddComment             = CapabilityComponent(2)
	CanEditContent            = CapabilityComponent(3)
	CanSeeOtherClients        = CapabilityComponent(5)
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
	if !c.Includes(action) {
		return c
	}
	return Capabilities(int(c) / int(action))
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
	Blob       []byte
	Msg        *websocket.PreparedMessage
	FatalError bool
}

type WriteQueue chan<- WriteQueueEntry

// generatePublicId yields a secure unique id
// It contains a 16 hex char long timestamp in ns precision, a hyphen and
// another 16 hex char long random string.
func generatePublicId() (sharedTypes.PublicId, error) {
	buf := make([]byte, 8)
	if _, err := secureRand.Read(buf); err != nil {
		return "", err
	}
	now := time.Now().UnixNano()
	id := sharedTypes.PublicId(
		strconv.FormatInt(now, 16) + "-" + hex.EncodeToString(buf),
	)
	return id, nil
}

func NewClient(projectId sharedTypes.UUID, user User, writerChanges chan bool, writeQueue WriteQueue, disconnect func()) (*Client, error) {
	publicId, err := generatePublicId()
	if err != nil {
		return nil, err
	}
	return &Client{
		lockedProjectId: projectId,
		PublicId:        publicId,
		User:            user,
		writerChanges:   writerChanges,
		writeQueue:      writeQueue,
		disconnect:      disconnect,
	}, nil
}

type Client struct {
	capabilities    Capabilities
	lockedProjectId sharedTypes.UUID

	DocId     sharedTypes.UUID
	PublicId  sharedTypes.PublicId
	ProjectId *sharedTypes.UUID
	User      User

	knownDocs []sharedTypes.UUID

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

func (c *Client) IsKnownDoc(id sharedTypes.UUID) bool {
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

func (c *Client) AddKnownDoc(id sharedTypes.UUID) {
	if len(c.knownDocs) < MaxKnownDocsToKeep {
		c.knownDocs = append(c.knownDocs, id)
	} else {
		c.knownDocs[weakRand.Int63n(MaxKnownDocsToKeep)] = id
	}
}

func (c *Client) ResolveCapabilities(privilegeLevel sharedTypes.PrivilegeLevel, isRestrictedUser project.IsRestrictedUser) {
	switch privilegeLevel {
	case sharedTypes.PrivilegeLevelOwner, sharedTypes.PrivilegeLevelReadAndWrite:
		c.capabilities = Capabilities(
			CanAddComment *
				CanEditContent *
				CanSeeOtherClients *
				CanSeeNonRestrictedEvents *
				CanSeeAllEditorEvents,
		)
	case sharedTypes.PrivilegeLevelReadOnly:
		c.capabilities = Capabilities(
			CanAddComment *
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
	if c.DocId == (sharedTypes.UUID{}) {
		return &errors.InvalidStateError{Msg: "join doc first"}
	}
	return nil
}

func (c *Client) CanJoinProject(id sharedTypes.UUID) error {
	if id != c.lockedProjectId {
		return errors.Tag(
			&errors.NotAuthorizedError{},
			"rejecting cross project join "+id.String(),
		)
	}
	return nil
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
	case JoinProject:
		if c.ProjectId != nil {
			return &errors.InvalidStateError{Msg: "already joined project"}
		}
		return nil
	case JoinDoc:
		if err := c.requireJoinedProject(); err != nil {
			return err
		}
		if c.DocId != (sharedTypes.UUID{}) && c.DocId != docId {
			return &errors.InvalidStateError{Msg: "leave other doc first"}
		}
		return nil
	case LeaveDoc:
		if err := c.requireJoinedProject(); err != nil {
			return err
		}
		if c.DocId != docId {
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
		if c.DocId != docId {
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
	return c.QueueMessage(WriteQueueEntry{
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
