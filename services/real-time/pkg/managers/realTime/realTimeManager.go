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
	"github.com/das7pad/real-time/pkg/managers/realTime/internal/documentUpdater"
	"github.com/das7pad/real-time/pkg/managers/realTime/internal/editorEvents"
	"github.com/das7pad/real-time/pkg/managers/realTime/internal/webApi"
	"github.com/das7pad/real-time/pkg/types"
)

type Manager interface {
	PeriodicCleanup(ctx context.Context)

	RPC(rpc *types.RPC)
	Disconnect(client *types.Client) error
}

func New(ctx context.Context, options *types.Options, client redis.UniversalClient) (Manager, error) {
	a, err := appliedOps.New(ctx, options, client)
	if err != nil {
		return nil, err
	}
	e, err := editorEvents.New(ctx, client)
	if err != nil {
		return nil, err
	}
	w, err := webApi.New(options)
	if err != nil {
		return nil, err
	}
	d, err := documentUpdater.New(options)
	if err != nil {
		return nil, err
	}
	return &manager{
		options:         options,
		appliedOps:      a,
		editorEvents:    e,
		documentUpdater: d,
		webApi:          w,
	}, nil
}

type manager struct {
	options *types.Options

	appliedOps      appliedOps.Manager
	editorEvents    editorEvents.Manager
	documentUpdater documentUpdater.Manager
	webApi          webApi.Manager
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
	cause := errors.GetCause(err)
	if potentiallyFatalError, ok := cause.(errors.PotentiallyFatalError); ok {
		rpc.Response.FatalError = potentiallyFatalError.IsFatal()
	}
	if publicError, ok := cause.(errors.PublicError); ok {
		rpc.Response.Error = publicError.Public()
	} else {
		rpc.Response.Error = &errors.JavaScriptError{
			Message: "Something went wrong in real-time service",
		}
	}
}

func (m *manager) joinProject(rpc *types.RPC) error {
	var args types.JoinProjectRequest
	if err := json.Unmarshal(rpc.Request.Body, &args); err != nil {
		return &errors.ValidationError{Msg: "bad request: " + err.Error()}
	}
	if err := rpc.Client.CanJoinProject(args.ProjectId); err != nil {
		return errors.Tag(
			err,
			"rejection cross project join "+args.ProjectId.Hex(),
		)
	}

	r, err := m.webApi.JoinProject(rpc, rpc.Client, &args)
	if err != nil {
		return errors.Tag(
			err, "webApi.joinProject failed for "+args.ProjectId.Hex(),
		)
	}
	rpc.Client.ResolveCapabilities(r.PrivilegeLevel, r.IsRestrictedUser)

	// For cleanup purposes: mark as joined before actually joining.
	rpc.Client.ProjectId = &args.ProjectId

	if err = m.editorEvents.Join(rpc, rpc.Client, args.ProjectId); err != nil {
		return errors.Tag(
			err, "editorEvents.Join failed for "+args.ProjectId.Hex(),
		)
	}

	levelRaw, err := json.Marshal(r.PrivilegeLevel)
	if err != nil {
		return errors.Tag(
			err,
			"encoding PrivilegeLevel failed for "+args.ProjectId.Hex(),
		)
	}
	res := types.JoinProjectResponse{
		r.Project,
		json.RawMessage(levelRaw),
		json.RawMessage("5"),
	}
	body, err := json.Marshal(res)
	if err != nil {
		return errors.Tag(
			err,
			"encoding JoinProjectResponse failed for "+args.ProjectId.Hex(),
		)
	}
	rpc.Response.Body = body
	return nil
}
func (m *manager) joinDoc(rpc *types.RPC) error {
	var args types.JoinDocRequest
	if err := json.Unmarshal(rpc.Request.Body, &args); err != nil {
		return &errors.ValidationError{Msg: "bad request: " + err.Error()}
	}
	args.DocId = rpc.Request.DocId

	r, err := m.documentUpdater.JoinDoc(rpc, rpc.Client, &args)
	if err != nil {
		return errors.Tag(
			err, "documentUpdater.JoinDoc failed for "+args.DocId.Hex(),
		)
	}
	if !rpc.Client.HasCapability(types.CanSeeComments) {
		r.Ranges.Comments = types.Comments{}
	}

	// For cleanup purposes: mark as joined before actually joining.
	rpc.Client.DocId = &args.DocId

	if err = m.appliedOps.Join(rpc, rpc.Client, args.DocId); err != nil {
		return errors.Tag(
			err, "appliedOps.Join failed for "+args.DocId.Hex(),
		)
	}

	body, err := json.Marshal(r)
	if err != nil {
		return errors.Tag(
			err,
			"encoding JoinDocResponse failed for "+args.DocId.Hex(),
		)
	}
	rpc.Response.Body = body
	return nil
}
func (m *manager) leaveDoc(rpc *types.RPC) error {
	docId := rpc.Client.DocId
	if docId == nil {
		// Ignore not yet joined.
		return nil
	}
	err := m.appliedOps.Leave(rpc.Client, *docId)
	if err != nil {
		return errors.Tag(
			err, "appliedOps.Leave failed for "+docId.Hex(),
		)
	}
	rpc.Client.DocId = nil
	return nil
}

const (
	maxUpdateSize = 7*1024*1024 + 64*1024
)

func (m *manager) preProcessApplyUpdateRequest(rpc *types.RPC) (*types.ApplyUpdateRequest, error) {
	if len(rpc.Request.Body) > maxUpdateSize {
		// Accept the update RPC at first.
		rpc.Client.WriteQueue <- &types.RPCResponse{
			Callback: rpc.Request.Callback,
		}

		// Then fire an otUpdateError.
		codedError := errors.CodedError{
			Description: "update is too large",
			Code:        "otUpdateError",
		}
		// Turn into broadcast.
		rpc.Response.Callback = 0

		rpc.Response.FatalError = true
		return nil, &codedError
	}
	var args types.ApplyUpdateRequest
	if err := json.Unmarshal(rpc.Request.Body, &args); err != nil {
		return nil, &errors.ValidationError{Msg: "bad request: " + err.Error()}
	}
	args.DocId = rpc.Request.DocId
	args.Meta.Source = rpc.Client.PublicId
	args.Meta.UserId = rpc.Client.User.Id
	if err := args.Validate(); err != nil {
		return nil, err
	}
	return &args, nil
}

func (m *manager) applyUpdate(rpc *types.RPC) error {
	args, err := m.preProcessApplyUpdateRequest(rpc)
	if err != nil {
		return err
	}
	return m.appliedOps.QueueUpdate(rpc, args)
}
func (m *manager) addComment(rpc *types.RPC) error {
	args, err := m.preProcessApplyUpdateRequest(rpc)
	if err != nil {
		return err
	}
	for _, op := range args.Ops {
		if !op.IsComment() {
			return &errors.NotAuthorizedError{}
		}
	}
	return m.appliedOps.QueueUpdate(rpc, args)
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
