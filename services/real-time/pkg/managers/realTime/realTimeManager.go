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
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/managers/realTime/internal/appliedOps"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/managers/realTime/internal/clientTracking"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/managers/realTime/internal/documentUpdater"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/managers/realTime/internal/editorEvents"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/managers/realTime/internal/webApi"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
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
	c := clientTracking.New(client)
	e, err := editorEvents.New(ctx, client, c)
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
		clientTracking:  c,
		editorEvents:    e,
		documentUpdater: d,
		webApi:          w,
	}, nil
}

type manager struct {
	options *types.Options

	clientTracking  clientTracking.Manager
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
		"user %s: %s: %s",
		rpc.Client.User.Id.Hex(), rpc.Request.Action, err.Error(),
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
			"rejecting cross project join "+args.ProjectId.Hex(),
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

	go func() {
		// Mark the user as joined in the background.
		// NOTE: UpdateClientPosition expects a present client.ProjectId.
		//       Start the goroutine after assigning one.
		m.clientTracking.InitializeClientPosition(rpc.Client)
	}()

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

	err := m.documentUpdater.CheckDocExists(rpc, rpc.Client, &args)
	if err != nil {
		return errors.Tag(
			err, "documentUpdater.CheckDocExists failed for "+args.DocId.Hex(),
		)
	}

	// For cleanup purposes: mark as joined before actually joining.
	rpc.Client.DocId = &args.DocId

	if err = m.appliedOps.Join(rpc, rpc.Client, args.DocId); err != nil {
		return errors.Tag(
			err, "appliedOps.Join failed for "+args.DocId.Hex(),
		)
	}

	r, err := m.documentUpdater.JoinDoc(rpc, rpc.Client, &args)
	if err != nil {
		return errors.Tag(
			err, "documentUpdater.JoinDoc failed for "+args.DocId.Hex(),
		)
	}
	if !rpc.Client.HasCapability(types.CanSeeComments) {
		r.Ranges.Comments = types.Comments{}
	}

	ranges, err := json.Marshal(r.Ranges)
	if err != nil {
		return errors.Tag(
			err,
			"encoding ranges failed for "+args.DocId.Hex(),
		)
	}
	body := []json.RawMessage{
		r.Snapshot,
		json.RawMessage(strconv.FormatInt(int64(r.Version), 10)),
		r.Ops,
		ranges,
	}
	blob, err := json.Marshal(body)
	if err != nil {
		return errors.Tag(
			err,
			"encoding JoinDocResponse failed for "+args.DocId.Hex(),
		)
	}
	rpc.Response.Body = blob
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

func (m *manager) preProcessApplyUpdateRequest(rpc *types.RPC) (*types.DocumentUpdate, error) {
	if len(rpc.Request.Body) > maxUpdateSize {
		// Accept the update RPC at first, keep going on error.
		_ = rpc.Client.QueueResponse(&types.RPCResponse{
			Callback: rpc.Request.Callback,
		})

		// Then fire an otUpdateError.
		codedError := &errors.CodedError{
			Description: "update is too large",
			Code:        "otUpdateError",
		}
		// Turn into broadcast.
		rpc.Response.Callback = 0
		rpc.Response.Name = "otUpdateError"

		rpc.Response.FatalError = true
		return nil, codedError
	}
	var args types.DocumentUpdate
	if err := json.Unmarshal(rpc.Request.Body, &args); err != nil {
		return nil, &errors.ValidationError{Msg: "bad request: " + err.Error()}
	}
	// Hard code document and user identifier.
	args.DocId = rpc.Request.DocId
	args.Meta.Source = rpc.Client.PublicId
	args.Meta.UserId = rpc.Client.User.Id
	// Dup is an output only field
	args.Dup = false
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
	if args.Ops.HasEditOp() {
		return &errors.NotAuthorizedError{}
	}
	return m.appliedOps.QueueUpdate(rpc, args)
}
func (m *manager) getConnectedUsers(rpc *types.RPC) error {
	users, err := m.clientTracking.GetConnectedClients(rpc, rpc.Client)
	if err != nil {
		return err
	}
	blob, err := json.Marshal(users)
	if err != nil {
		return errors.Tag(err, "cannot serialize users")
	}
	body, err := json.Marshal([]json.RawMessage{blob})
	if err != nil {
		return errors.Tag(err, "cannot wrap users")
	}
	rpc.Response.Body = body
	return nil
}
func (m *manager) updatePosition(rpc *types.RPC) error {
	var args types.ClientPosition
	if err := json.Unmarshal(rpc.Request.Body, &args); err != nil {
		return &errors.ValidationError{Msg: "bad request: " + err.Error()}
	}
	// Hard code document identifier.
	args.DocId = rpc.Request.DocId

	err := m.clientTracking.UpdateClientPosition(rpc, rpc.Client, &args)
	if err != nil {
		return errors.Tag(err, "cannot persist position update")
	}

	notification := types.ClientPositionUpdateNotification{
		Source: rpc.Client.PublicId,
		Row:    args.Row,
		Column: args.Column,
		DocId:  args.DocId,
	}
	body, err := json.Marshal(notification)
	if err != nil {
		return errors.Tag(err, "cannot encode notification")
	}
	msg := types.EditorEventsMessage{
		RoomId:  *rpc.Client.ProjectId,
		Message: editorEvents.ClientTrackingClientUpdated,
		Payload: body,
	}
	if err = m.editorEvents.Broadcast(rpc, &msg); err != nil {
		return errors.Tag(err, "cannot send notification")
	}
	if rpc.Request.Callback == 0 {
		rpc.Response = nil
	}
	return nil
}
func (m *manager) Disconnect(client *types.Client) error {
	var errAppliedOps, errEditorEvents, errClientTracking error
	docId := client.DocId
	if docId != nil {
		errAppliedOps = m.appliedOps.Leave(client, *docId)
	}
	projectId := client.ProjectId
	if projectId != nil {
		errEditorEvents = m.editorEvents.Leave(client, *projectId)

		// Skip cleanup when not joined yet.
		var nowEmpty bool
		nowEmpty, errClientTracking = m.cleanupClientTracking(client)
		// Flush eagerly on error.
		if nowEmpty || errClientTracking != nil {
			m.backgroundFlush(client)
		}
	}

	if errAppliedOps != nil {
		return errAppliedOps
	}
	if errEditorEvents != nil {
		return errEditorEvents
	}
	if errClientTracking != nil {
		return errClientTracking
	}
	return nil
}

func (m *manager) cleanupClientTracking(client *types.Client) (bool, error) {
	nowEmpty := m.clientTracking.DeleteClientPosition(client)

	body := json.RawMessage("\"" + client.PublicId + "\"")
	msg := types.EditorEventsMessage{
		RoomId:  *client.ProjectId,
		Message: "clientTracking.clientDisconnected",
		Payload: body,
	}
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	if err := m.editorEvents.Broadcast(ctx, &msg); err != nil {
		return nowEmpty, errors.Tag(
			err, "cannot send notification for disconnect",
		)
	}
	return nowEmpty, nil
}

func (m *manager) backgroundFlush(client *types.Client) {
	ctx, done := context.WithTimeout(context.Background(), 30*time.Second)
	defer done()

	err := m.documentUpdater.FlushProject(ctx, client)
	if err != nil {
		log.Println(
			errors.Tag(
				err, "background flush failed for "+client.ProjectId.Hex(),
			).Error(),
		)
	}
}

func (m *manager) rpc(rpc *types.RPC) error {
	if err := rpc.Validate(); err != nil {
		return err
	}

	switch rpc.Request.Action {
	case types.Ping:
		return nil
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