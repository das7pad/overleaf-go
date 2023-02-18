// Golang port of Overleaf
// Copyright (C) 2021-2023 Jakob Ackermann <das7pad@outlook.com>
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
	"sync/atomic"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/jwt/projectJWT"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/managers/realTime/internal/appliedOps"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/managers/realTime/internal/clientTracking"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/managers/realTime/internal/editorEvents"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/managers/realTime/internal/webApi"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

type Manager interface {
	InitiateGracefulShutdown()
	TriggerGracefulReconnect()
	IsShuttingDown() bool

	PeriodicCleanup(ctx context.Context)

	BootstrapWS(ctx context.Context, client *types.Client, claims projectJWT.Claims) ([]byte, error)
	RPC(ctx context.Context, rpc *types.RPC)
	Disconnect(client *types.Client) error
}

func New(ctx context.Context, options *types.Options, db *pgxpool.Pool, client redis.UniversalClient, dum documentUpdater.Manager) (Manager, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}

	a := appliedOps.New(client, dum)
	if err := a.StartListening(ctx); err != nil {
		return nil, err
	}
	ct := clientTracking.New(client)
	e := editorEvents.New(client, ct)
	if err := e.StartListening(ctx); err != nil {
		return nil, err
	}
	w := webApi.New(db)
	return &manager{
		appliedOps:              a,
		clientTracking:          ct,
		editorEvents:            e,
		documentUpdater:         dum,
		webApi:                  w,
		gracefulShutdownDelay:   options.GracefulShutdown.Delay,
		gracefulShutdownTimeout: options.GracefulShutdown.Timeout,
	}, nil
}

type manager struct {
	shuttingDown atomic.Bool

	clientTracking  clientTracking.Manager
	appliedOps      appliedOps.Manager
	editorEvents    editorEvents.Manager
	documentUpdater documentUpdater.Manager
	webApi          webApi.Manager

	gracefulShutdownDelay   time.Duration
	gracefulShutdownTimeout time.Duration
}

func (m *manager) IsShuttingDown() bool {
	return m.shuttingDown.Load()
}

func (m *manager) PeriodicCleanup(ctx context.Context) {
	<-ctx.Done()
}

func (m *manager) InitiateGracefulShutdown() {
	// Start returning 500s on /status
	m.shuttingDown.Store(true)

	// Wait for the LB to pick up the 500 and stop sending new traffic to us.
	time.Sleep(m.gracefulShutdownDelay)
}

func (m *manager) TriggerGracefulReconnect() {
	deadLine := time.Now().Add(m.gracefulShutdownTimeout)
	for m.editorEvents.TriggerGracefulReconnect() > 0 &&
		time.Now().Before(deadLine) {

		time.Sleep(3 * time.Second)
	}
}

func (m *manager) RPC(ctx context.Context, rpc *types.RPC) {
	err := m.rpc(ctx, rpc)
	if err == nil {
		return
	}
	log.Printf(
		"user %s: %s: %s",
		rpc.Client.User.Id.String(), rpc.Request.Action, err.Error(),
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

func (m *manager) BootstrapWS(ctx context.Context, client *types.Client, claims projectJWT.Claims) ([]byte, error) {
	u, r, err := m.webApi.BootstrapWS(ctx, claims)
	if err != nil {
		return nil, err
	}
	projectId := r.Project.Id

	client.ProjectId = projectId
	client.User = u
	client.ResolveCapabilities(r.PrivilegeLevel, r.IsRestrictedUser)

	// Fetch connected users in the background.
	var connectedClients types.ConnectedClients
	fetchUsersCtx, doneFetchingUsers := context.WithTimeout(ctx, time.Second)
	if client.CanDo(types.GetConnectedUsers, sharedTypes.UUID{}) == nil {
		defer doneFetchingUsers()
		go func() {
			connectedClients, _ = m.clientTracking.GetConnectedClients(
				fetchUsersCtx, client,
			)
			doneFetchingUsers()
		}()
	} else {
		connectedClients = make(types.ConnectedClients, 0)
		doneFetchingUsers()
	}

	go func() {
		// Mark the user as joined in the background.
		m.clientTracking.InitializeClientPosition(client)
	}()

	if err = m.editorEvents.Join(ctx, client, projectId); err != nil {
		return nil, errors.Tag(
			err, "editorEvents.Join failed for "+projectId.String(),
		)
	}

	// Wait for the fetch, but ignore any fetch errors.
	// Instead, let the client fetch any connectedClients via a 2nd rpc call.
	<-fetchUsersCtx.Done()

	res := &types.JoinProjectResponse{
		Project:          r.Project,
		PrivilegeLevel:   r.PrivilegeLevel,
		ConnectedClients: connectedClients,
		PublicId:         client.PublicId,
	}
	body, err := json.Marshal(res)
	if err != nil {
		return nil, errors.Tag(
			err,
			"encoding JoinProjectResponse failed for "+projectId.String(),
		)
	}
	return body, nil
}

func (m *manager) joinDoc(ctx context.Context, rpc *types.RPC) error {
	var args types.JoinDocRequest
	if err := json.Unmarshal(rpc.Request.Body, &args); err != nil {
		return &errors.ValidationError{Msg: "bad request: " + err.Error()}
	}
	args.DocId = rpc.Request.DocId

	if !rpc.Client.IsKnownDoc(rpc.Request.DocId) {
		err := m.documentUpdater.CheckDocExists(
			ctx,
			rpc.Client.ProjectId,
			args.DocId,
		)
		if err != nil {
			return errors.Tag(
				err,
				"documentUpdater.CheckDocExists failed for "+args.DocId.String(),
			)
		}
		rpc.Client.AddKnownDoc(rpc.Request.DocId)
	}

	// For cleanup purposes: mark as joined before actually joining.
	rpc.Client.DocId = args.DocId

	if err := m.appliedOps.Join(ctx, rpc.Client, args.DocId); err != nil {
		return errors.Tag(
			err, "appliedOps.Join failed for "+args.DocId.String(),
		)
	}

	r, err := m.documentUpdater.GetDoc(
		ctx,
		rpc.Client.ProjectId,
		args.DocId,
		args.FromVersion,
	)
	if err != nil {
		return errors.Tag(
			err, "documentUpdater.GetDoc failed for "+args.DocId.String(),
		)
	}

	body := &types.JoinDocResponse{
		Snapshot: sharedTypes.Snapshot(r.Snapshot),
		Version:  r.Version,
		Updates:  r.Ops,
	}
	blob, err := json.Marshal(body)
	if err != nil {
		return errors.Tag(
			err,
			"encoding JoinDocResponse failed for "+args.DocId.String(),
		)
	}
	rpc.Response.Body = blob
	return nil
}

func (m *manager) leaveDoc(rpc *types.RPC) error {
	docId := rpc.Request.DocId
	err := m.appliedOps.Leave(rpc.Client, docId)
	if err != nil {
		return errors.Tag(
			err, "appliedOps.Leave failed for "+docId.String(),
		)
	}
	rpc.Client.DocId = sharedTypes.UUID{}
	return nil
}

const (
	maxUpdateSize = 7*1024*1024 + 64*1024
)

func (m *manager) applyUpdate(ctx context.Context, rpc *types.RPC) error {
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
		return codedError
	}
	var args sharedTypes.DocumentUpdate
	if err := json.Unmarshal(rpc.Request.Body, &args); err != nil {
		return &errors.ValidationError{Msg: "bad request: " + err.Error()}
	}
	// Hard code document and user identifier.
	args.DocId = rpc.Request.DocId
	args.Meta.Source = rpc.Client.PublicId
	args.Meta.UserId = rpc.Client.User.Id
	// Dup is an output only field
	args.Dup = false
	// Ingestion time is tracked internally only
	now := time.Now()
	args.Meta.IngestionTime = &now

	if err := args.Validate(); err != nil {
		return err
	}
	return m.appliedOps.QueueUpdate(ctx, rpc, &args)
}

func (m *manager) getConnectedUsers(ctx context.Context, rpc *types.RPC) error {
	clients, err := m.clientTracking.GetConnectedClients(ctx, rpc.Client)
	if err != nil {
		return err
	}
	body := types.GetConnectedUsersResponse{ConnectedClients: clients}
	blob, err := json.Marshal(body)
	if err != nil {
		return errors.Tag(err, "cannot serialize users")
	}
	rpc.Response.Body = blob
	return nil
}

func (m *manager) updatePosition(ctx context.Context, rpc *types.RPC) error {
	var args types.ClientPosition
	if err := json.Unmarshal(rpc.Request.Body, &args); err != nil {
		return &errors.ValidationError{Msg: "bad request: " + err.Error()}
	}
	// Hard code document identifier.
	args.DocId = rpc.Request.DocId

	err := m.clientTracking.UpdateClientPosition(ctx, rpc.Client, &args)
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
	msg := &sharedTypes.EditorEventsMessage{
		RoomId:  rpc.Client.ProjectId,
		Message: editorEvents.ClientTrackingClientUpdated,
		Payload: body,
	}
	if err = m.editorEvents.Publish(ctx, msg); err != nil {
		return errors.Tag(err, "cannot send notification")
	}
	if rpc.Request.Callback == 0 {
		rpc.Response = nil
	}
	return nil
}

func (m *manager) Disconnect(client *types.Client) error {
	var errAppliedOps, errEditorEvents, errClientTracking error
	if !client.DocId.IsZero() {
		errAppliedOps = m.appliedOps.Leave(client, client.DocId)
	}
	if !client.ProjectId.IsZero() {
		errEditorEvents = m.editorEvents.Leave(client, client.ProjectId)

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
	msg := &sharedTypes.EditorEventsMessage{
		RoomId:  client.ProjectId,
		Message: "clientTracking.clientDisconnected",
		Payload: body,
	}
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	if err := m.editorEvents.Publish(ctx, msg); err != nil {
		return nowEmpty, errors.Tag(
			err, "cannot send notification for disconnect",
		)
	}
	return nowEmpty, nil
}

func (m *manager) backgroundFlush(client *types.Client) {
	ctx, done := context.WithTimeout(context.Background(), 30*time.Second)
	defer done()

	err := m.documentUpdater.FlushProject(ctx, client.ProjectId)
	if err != nil {
		log.Println(
			errors.Tag(
				err, "background flush failed for "+client.ProjectId.String(),
			).Error(),
		)
	}
}

func (m *manager) rpc(ctx context.Context, rpc *types.RPC) error {
	if err := rpc.Validate(); err != nil {
		return err
	}

	switch rpc.Request.Action {
	case types.Ping:
		return nil
	case types.JoinDoc:
		return m.joinDoc(ctx, rpc)
	case types.LeaveDoc:
		return m.leaveDoc(rpc)
	case types.ApplyUpdate:
		return m.applyUpdate(ctx, rpc)
	case types.GetConnectedUsers:
		return m.getConnectedUsers(ctx, rpc)
	case types.UpdatePosition:
		return m.updatePosition(ctx, rpc)
	default:
		return &errors.ValidationError{
			Msg: "unknown action: " + string(rpc.Request.Action),
		}
	}
}
