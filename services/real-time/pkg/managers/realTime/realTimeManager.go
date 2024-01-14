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

package realTime

import (
	"context"
	"encoding/json"
	"log"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/das7pad/overleaf-go/pkg/cache"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/jwt/projectJWT"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/user"
	"github.com/das7pad/overleaf-go/pkg/pubSub/channel"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/managers/realTime/internal/clientTracking"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/managers/realTime/internal/editorEvents"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

type Manager interface {
	InitiateGracefulShutdown()
	TriggerGracefulReconnect()
	DisconnectAll()
	IsShuttingDown() bool
	PeriodicCleanup(ctx context.Context)
	BootstrapWS(ctx context.Context, resp *types.RPCResponse, client *types.Client, claims projectJWT.Claims) error
	RPC(ctx context.Context, rpc *types.RPC)
	Disconnect(client *types.Client)
}

func New(ctx context.Context, options *types.Options, db *pgxpool.Pool, client redis.UniversalClient, dum documentUpdater.Manager) (Manager, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}

	c := channel.New(client, "editor-events")
	ct := clientTracking.New(client, c)
	e := editorEvents.New(c, ct.FlushRoomChanges, dum.FlushProjectInBackground)
	if err := e.StartListening(ctx); err != nil {
		return nil, err
	}
	pc := cache.NewLimited[projectCacheKey, json.RawMessage](100)
	return &manager{
		clientTracking:   ct,
		editorEvents:     e,
		dum:              dum,
		pm:               project.New(db),
		gracefulShutdown: options.GracefulShutdown,
		projectCache:     pc,
	}, nil
}

type projectCacheKey struct {
	ProjectId        sharedTypes.UUID
	ProjectEpoch     int64
	AccessSourceEnum int8
}

type manager struct {
	shuttingDown atomic.Bool

	clientTracking clientTracking.Manager
	editorEvents   editorEvents.Manager
	dum            documentUpdater.Manager
	pm             project.Manager
	projectCache   *cache.Limited[projectCacheKey, json.RawMessage]

	gracefulShutdown types.GracefulShutdownOptions
}

func (m *manager) IsShuttingDown() bool {
	return m.shuttingDown.Load()
}

func (m *manager) PeriodicCleanup(ctx context.Context) {
	jitter := time.Duration(rand.Int63n(int64(30 * time.Second)))
	inter := clientTracking.RefreshUserEvery - jitter
	t := time.NewTicker(inter)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			clients := m.editorEvents.GetClients()
			err := m.clientTracking.RefreshClientPositions(ctx, clients)
			if err != nil {
				log.Printf("refresh client positions: %s", err)
			}
		}
	}
}

func (m *manager) InitiateGracefulShutdown() {
	// Start returning 500s on /status
	m.shuttingDown.Store(true)

	// Wait for the LB to pick up the 500 and stop sending new traffic to us.
	time.Sleep(m.gracefulShutdown.Delay)
}

func (m *manager) TriggerGracefulReconnect() {
	deadLine := time.Now().Add(m.gracefulShutdown.Timeout)
	for m.triggerGracefulReconnectOnce() && time.Now().Before(deadLine) {
		time.Sleep(3 * time.Second)
	}
}

func (m *manager) DisconnectAll() {
	deadLine := time.Now().Add(m.gracefulShutdown.CleanupTimeout)
	for time.Now().Before(deadLine) {
		rooms := m.editorEvents.GetClients()
		if len(rooms) == 0 {
			break
		}
		for _, clients := range rooms {
			for i, client := range clients.All {
				if i != clients.Removed {
					client.TriggerDisconnect()
				}
			}
		}
		time.Sleep(time.Second)
	}
}

const b64Chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"

func (m *manager) triggerGracefulReconnectOnce() bool {
	for _, c := range b64Chars {
		suffix := uint8(c)
		nRooms := m.editorEvents.BroadcastGracefulReconnect(suffix)
		if nRooms == 0 {
			return false
		}
		const targetReconnectsPerSecond = 1000
		const delayPerClient = time.Second / targetReconnectsPerSecond
		const clientsPerRoomP95 = 1
		const reconnectCycles = len(b64Chars)
		time.Sleep(
			time.Duration(nRooms) * clientsPerRoomP95 * delayPerClient /
				time.Duration(reconnectCycles),
		)
	}
	return true
}

func (m *manager) RPC(ctx context.Context, rpc *types.RPC) {
	err := m.rpc(ctx, rpc)
	if err == nil {
		return
	}
	log.Printf(
		"user=%s project=%s doc=%s action=%s body=%d err=%s",
		rpc.Client.UserId, rpc.Client.ProjectId, rpc.Request.DocId,
		rpc.Request.Action, len(rpc.Request.Body), err.Error(),
	)
	if errors.IsFatalError(err) {
		rpc.Response.FatalError = true
	}
	rpc.Response.Error = &errors.JavaScriptError{
		Message: errors.GetPublicMessage(
			err, "Something went wrong in real-time service",
		),
	}
}

var emptyConnectedClients = json.RawMessage("[]")

func (m *manager) BootstrapWS(ctx context.Context, resp *types.RPCResponse, client *types.Client, claims projectJWT.Claims) error {
	res := types.BootstrapWSResponse{
		PrivilegeLevel: claims.PrivilegeLevel,
		PublicId:       client.PublicId,
	}
	u := user.WithPublicInfo{}
	cacheKey := projectCacheKey{
		ProjectId:        claims.ProjectId,
		ProjectEpoch:     claims.Epoch,
		AccessSourceEnum: claims.AccessSource.Enum(),
	}
	if cached, ok := m.projectCache.Get(cacheKey); ok {
		err := m.pm.GetBootstrapWSUser(
			ctx, claims.ProjectId, claims.UserId,
			claims.Epoch, claims.EpochUser,
			&u,
		)
		if err != nil {
			return err
		}
		res.Project = cached
	} else {
		p := types.ProjectDetails{}
		err := m.pm.GetBootstrapWSDetails(
			ctx, claims.ProjectId, claims.UserId,
			claims.Epoch, claims.EpochUser,
			claims.AccessSource, &p.ForBootstrapWS, &u,
		)
		if err != nil {
			return err
		}
		p.Id = claims.ProjectId
		t := claims.Timeout / sharedTypes.ComputeTimeout(time.Second)
		p.OwnerFeatures = user.Features{
			Collaborators:  -1,
			CompileTimeout: t,
			CompileGroup:   claims.CompileGroup,
			Versioning:     true,
		}
		p.RootFolder = []*project.Folder{p.GetRootFolder()}
		if cached, err = json.Marshal(p); err != nil {
			return err
		}
		m.projectCache.Add(cacheKey, cached)
		res.Project = cached
	}

	client.DisplayName = u.DisplayName()
	client.ProjectId = claims.ProjectId
	client.UserId = claims.UserId
	client.ResolveCapabilities(claims.PrivilegeLevel, claims.IsRestrictedUser())

	if err := m.editorEvents.Join(ctx, client); err != nil {
		return errors.Tag(err, "subscribe")
	}

	if client.HasCapability(types.CanSeeOtherClients) {
		res.ConnectedClients, _ =
			m.clientTracking.GetConnectedClients(ctx, client)
	} else {
		res.ConnectedClients = emptyConnectedClients
	}

	res.WriteInto(resp)
	return nil
}

func (m *manager) joinDoc(ctx context.Context, rpc *types.RPC) error {
	var args types.JoinDocRequest
	if err := json.Unmarshal(rpc.Request.Body, &args); err != nil {
		return &errors.ValidationError{Msg: "bad request: " + err.Error()}
	}
	docId := rpc.Request.DocId

	r, err := m.dum.GetDoc(ctx, rpc.Client.ProjectId, docId, args.FromVersion)
	if err != nil {
		return errors.Tag(err, "get doc")
	}
	rpc.Client.MarkAsJoined(rpc.Request.DocId)

	body := types.JoinDocResponse{
		Snapshot: sharedTypes.Snapshot(r.Snapshot),
		Version:  r.Version,
		Updates:  r.Ops,
	}
	blob, err := json.Marshal(body)
	if err != nil {
		return errors.Tag(err, "serialize response")
	}
	rpc.Response.Body = blob
	return nil
}

func (m *manager) applyUpdate(ctx context.Context, rpc *types.RPC) error {
	var update sharedTypes.DocumentUpdate
	if err := json.Unmarshal(rpc.Request.Body, &update); err != nil {
		return &errors.ValidationError{Msg: "bad request: " + err.Error()}
	}
	// Hard code user identifier.
	update.Meta.Source = rpc.Client.PublicId
	update.Meta.UserId = rpc.Client.UserId

	return m.dum.QueueUpdate(
		ctx, rpc.Client.ProjectId, rpc.Request.DocId, update,
	)
}

func (m *manager) getConnectedUsers(ctx context.Context, rpc *types.RPC) error {
	clients, err := m.clientTracking.GetConnectedClients(ctx, rpc.Client)
	if err != nil {
		return err
	}
	body := types.GetConnectedUsersResponse{ConnectedClients: clients}
	blob, err := body.MarshalJSON()
	if err != nil {
		return errors.Tag(err, "serialize users")
	}
	rpc.Response.Body = blob
	return nil
}

func (m *manager) updatePosition(ctx context.Context, rpc *types.RPC) error {
	var p types.ClientPosition
	if err := json.Unmarshal(rpc.Request.Body, &p); err != nil {
		return &errors.ValidationError{Msg: "bad request: " + err.Error()}
	}
	if err := m.clientTracking.UpdatePosition(ctx, rpc.Client, p); err != nil {
		return errors.Tag(err, "handle position update")
	}
	return nil
}

func (m *manager) Disconnect(client *types.Client) {
	if client.ProjectId.IsZero() {
		// Disconnected before bootstrap finished.
		client.CloseWriteQueue()
		return
	}
	client.MarkAsLeftDoc()
	m.editorEvents.Leave(client)
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
