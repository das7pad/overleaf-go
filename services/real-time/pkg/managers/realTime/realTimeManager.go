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

package realTime

import (
	"context"
	"encoding/json"
	"log"

	"github.com/go-redis/redis/v8"

	"github.com/das7pad/real-time/pkg/errors"
	"github.com/das7pad/real-time/pkg/managers/realTime/internal/appliedOps"
	"github.com/das7pad/real-time/pkg/managers/realTime/internal/editorEvents"
	"github.com/das7pad/real-time/pkg/managers/realTime/internal/webApiManager"
	"github.com/das7pad/real-time/pkg/types"
)

type Manager interface {
	PeriodicCleanup(ctx context.Context)

	RPC(rpc *types.RPC)
	Disconnect(client *types.Client) error
}

func New(ctx context.Context, options *types.Options, c redis.UniversalClient) (Manager, error) {
	a, err := appliedOps.New(ctx, c)
	if err != nil {
		return nil, err
	}
	e, err := editorEvents.New(ctx, c)
	if err != nil {
		return nil, err
	}
	w := webApiManager.New(options)
	return &manager{
		options:      options,
		appliedOps:   a,
		editorEvents: e,
		webApi:       w,
	}, nil
}

type manager struct {
	options *types.Options

	appliedOps   appliedOps.Manager
	editorEvents editorEvents.Manager
	webApi       webApiManager.Manager
}

func (m *manager) PeriodicCleanup(ctx context.Context) {
	<-ctx.Done()
}

func (m *manager) RPC(rpc *types.RPC) {
	err := m.rpc(rpc)
	if err == nil {
		return
	}
	log.Printf(
		"%s: %s: %s",
		rpc.Client.User.Id, rpc.Request.Action, err.Error(),
	)
	if errors.IsValidationError(err) {
		rpc.Response.Error = err.Error()
		return
	}
	if errors.IsInvalidState(err) {
		rpc.Response.Error = err.Error()
		rpc.Response.FatalError = true
		return
	}
	rpc.Response.Error = "Something went wrong in real-time service"
}

func (m *manager) joinProject(rpc *types.RPC) error {
	var args types.JoinProjectRequest
	if err := json.Unmarshal(rpc.Request.Body, &args); err != nil {
		return &errors.ValidationError{Msg: "bad request: " + err.Error()}
	}
	r, err := m.webApi.JoinProject(rpc, rpc.Client, &args)
	if err != nil {
		return errors.Tag(
			err, "webApi.joinProject failed for "+args.ProjectId.Hex(),
		)
	}
	rpc.Client.ResolveCapabilities(r.PrivilegeLevel, r.IsRestrictedUser)
	err = m.editorEvents.Join(rpc, rpc.Client, args.ProjectId)
	if err != nil {
		return errors.Tag(
			err, "editorEvents.Join failed for "+args.ProjectId.Hex(),
		)
	}
	res := types.JoinProjectResponse{
		r.Project,
		json.RawMessage(r.PrivilegeLevel),
		json.RawMessage("5"),
	}
	body, err := json.Marshal(res)
	if err != nil {
		return errors.Tag(
			err,
			"encoding joinProject response failed for "+args.ProjectId.Hex(),
		)
	}
	rpc.Response.Body = body
	return nil
}
func (m *manager) joinDoc(rpc *types.RPC) error {
	return nil
}
func (m *manager) leaveDoc(rpc *types.RPC) error {
	return nil
}
func (m *manager) applyUpdate(rpc *types.RPC) error {
	return m.appliedOps.ApplyUpdate(rpc)
}
func (m *manager) addComment(rpc *types.RPC) error {
	return m.appliedOps.AddComment(rpc)
}
func (m *manager) getConnectedUsers(rpc *types.RPC) error {
	return nil
}
func (m *manager) updatePosition(rpc *types.RPC) error {
	return nil
}
func (m *manager) Disconnect(client *types.Client) error {
	var errAppliedOps, errEditorEvents error
	docId := client.DocId
	if docId != nil {
		errAppliedOps = m.appliedOps.Leave(client, *docId)
	}
	projectId := client.ProjectId
	if projectId != nil {
		errEditorEvents = m.editorEvents.Leave(client, *projectId)
	}
	// TODO: delete client tracking entry?

	if errAppliedOps != nil {
		return errAppliedOps
	}
	if errEditorEvents != nil {
		return errEditorEvents
	}
	return nil
}

func (m *manager) rpc(rpc *types.RPC) error {
	if err := rpc.Validate(); err != nil {
		return err
	}

	switch rpc.Request.Action {
	case types.JoinProject:
		return m.joinProject(rpc)
	case types.JoinDoc:
		return m.joinDoc(rpc)
	case types.LeaveDoc:
		return m.leaveDoc(rpc)
	case types.ApplyUpdate:
		return m.applyUpdate(rpc)
	case types.AddComment:
		return m.addComment(rpc)
	case types.GetConnectedUsers:
		return m.getConnectedUsers(rpc)
	case types.UpdatePosition:
		return m.updatePosition(rpc)
	default:
		return &errors.ValidationError{
			Msg: "unknown action: " + string(rpc.Request.Action),
		}
	}
}
