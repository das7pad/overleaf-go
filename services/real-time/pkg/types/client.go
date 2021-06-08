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

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/real-time/pkg/errors"
)

type Capabilities int
type CapabilityComponent int

const (
	NoCapabilities     = Capabilities(0)
	CanAddComment      = CapabilityComponent(2)
	CanEditContent     = CapabilityComponent(3)
	CanSeeOtherClients = CapabilityComponent(5)
)

func (c Capabilities) CheckIncludes(action CapabilityComponent) error {
	if int(c)%int(action) != 0 {
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

func NewClient(wsBootstrap *WsBootstrap, writeQueue chan<- *RPCResponse) (*Client, error) {
	publicId, err := getPublicId()
	if err != nil {
		return nil, err
	}
	return &Client{
		lockedProjectId: wsBootstrap.ProjectId,
		PublicId:        publicId,
		User:            wsBootstrap.User,
		WriteQueue:      writeQueue,
	}, nil
}

type Client struct {
	capabilities    Capabilities
	lockedProjectId primitive.ObjectID

	DocId     *primitive.ObjectID
	PublicId  PublicId
	ProjectId *primitive.ObjectID
	User      *User

	WriteQueue chan<- *RPCResponse

	nextClientAppliedOps   *Client
	nextClientEditorEvents *Client
}

func GetNextAppliedOpsClient(client *Client) *Client {
	return client.nextClientAppliedOps
}
func GetNextEditorEventsClient(client *Client) *Client {
	return client.nextClientEditorEvents
}
func SetNextAppliedOpsClient(client *Client, next *Client) {
	client.nextClientAppliedOps = next
}
func SetNextEditorEventsClient(client *Client, next *Client) {
	client.nextClientEditorEvents = next
}

type PrivilegeLevel string
type IsRestrictedUser bool

func (c *Client) ResolveCapabilities(privilegeLevel PrivilegeLevel, isRestrictedUser IsRestrictedUser) {
	switch privilegeLevel {
	case "owner", "readAndWrite":
		c.capabilities = Capabilities(
			CanAddComment * CanEditContent * CanSeeOtherClients,
		)
	case "readOnly":
		c.capabilities = Capabilities(CanAddComment)
	default:
		c.capabilities = NoCapabilities
	}
	if isRestrictedUser {
		c.capabilities = c.capabilities.TakeAway(CanSeeOtherClients)
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

func (c *Client) CanDo(action Action, docId primitive.ObjectID) error {
	switch action {
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
		if err := c.requireJoinedProjectAndDoc(); err != nil {
			return err
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
