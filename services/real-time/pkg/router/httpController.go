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

package router

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/httpUtils"
	"github.com/das7pad/overleaf-go/pkg/jwt/jwtHandler"
	"github.com/das7pad/overleaf-go/pkg/jwt/projectJWT"
	"github.com/das7pad/overleaf-go/pkg/jwt/wsBootstrap"
	"github.com/das7pad/overleaf-go/pkg/options/jwtOptions"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/events"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/managers/realTime"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

func New(rtm realTime.Manager, jwtOptionsWsBootstrap, jwtOptionsProject jwtOptions.JWTOptions) *httpUtils.Router {
	r := httpUtils.NewRouter(&httpUtils.RouterOptions{
		Ready: func() bool {
			return !rtm.IsShuttingDown()
		},
	})
	Add(r, rtm, jwtOptionsWsBootstrap, jwtOptionsProject)
	return r
}

func Add(r *httpUtils.Router, rtm realTime.Manager, jwtOptionsWsBootstrap, jwtOptionsProject jwtOptions.JWTOptions) {
	(&httpController{
		rtm: rtm,
		u: websocket.Upgrader{
			Subprotocols: []string{
				protoV6,
				protoV7,
				protoV8,
			},
		},
		jwtWSBootstrap: wsBootstrap.New(jwtOptionsWsBootstrap),
		jwtProject: projectJWT.New(
			jwtOptionsProject,
			func(ctx context.Context, projectId, userId sharedTypes.UUID, projectEpoch, userEpoch int64) error {
				// validation is performed as part of bootstrap
				return &errors.NotAuthorizedError{}
			}),
	}).addRoutes(r)
}

type httpController struct {
	rtm            realTime.Manager
	u              websocket.Upgrader
	jwtWSBootstrap jwtHandler.JWTHandler
	jwtProject     jwtHandler.JWTHandler
}

const (
	protoV6                  = "v6.real-time.overleaf.com"
	protoV6JWTQueryParameter = "bootstrap"
	protoV7                  = "v7.real-time.overleaf.com"
	protoV7JWTProtoPrefix    = ".bootstrap.v7.real-time.overleaf.com"
	protoV8                  = "v8.real-time.overleaf.com"
	protoV8JWTProtoPrefix    = ".bootstrap.v8.real-time.overleaf.com"
)

func (h *httpController) addRoutes(router *httpUtils.Router) {
	router.GET("/socket.io", h.ws)
}

func (h *httpController) getProjectJWT(c *httpUtils.Context) (*projectJWT.Claims, error) {
	var blob string
	for _, proto := range websocket.Subprotocols(c.Request) {
		if strings.HasSuffix(proto, protoV8JWTProtoPrefix) {
			blob = proto[:len(proto)-len(protoV8JWTProtoPrefix)]
			break
		}
	}
	if len(blob) == 0 {
		return nil, &errors.ValidationError{Msg: "missing bootstrap blob"}
	}
	genericClaims, jwtError := h.jwtProject.Parse(blob)
	if jwtError != nil {
		return nil, jwtError
	}
	return genericClaims.(*projectJWT.Claims), nil
}

func (h *httpController) getWsBootstrap(c *httpUtils.Context) (*wsBootstrap.Claims, error) {
	var blob string
	for _, proto := range websocket.Subprotocols(c.Request) {
		if proto == protoV6 {
			blob = c.Request.URL.Query().Get(protoV6JWTQueryParameter)
			break
		}
		if strings.HasSuffix(proto, protoV7JWTProtoPrefix) {
			blob = proto[:len(proto)-len(protoV7JWTProtoPrefix)]
			break
		}
	}
	if len(blob) == 0 {
		return nil, &errors.ValidationError{Msg: "missing bootstrap blob"}
	}
	genericClaims, jwtError := h.jwtWSBootstrap.Parse(blob)
	if jwtError != nil {
		return nil, jwtError
	}
	return genericClaims.(*wsBootstrap.Claims), nil
}

func sendAndForget(conn *websocket.Conn, entry types.WriteQueueEntry) {
	_ = conn.WritePreparedMessage(entry.Msg)
}

func (h *httpController) ws(requestCtx *httpUtils.Context) {
	setupTime := sharedTypes.Timed{}
	setupTime.Begin()
	conn, upgradeErr := h.u.Upgrade(
		requestCtx.Writer, requestCtx.Request, nil,
	)
	if upgradeErr != nil {
		// A 4xx has been generated already.
		return
	}
	defer func() { _ = conn.Close() }()

	if h.rtm.IsShuttingDown() {
		sendAndForget(conn, events.ConnectionRejectedRetryPrepared)
		return
	}

	var claimsWSBootstrap *wsBootstrap.Claims
	var claimsProjectJWT *projectJWT.Claims
	switch conn.Subprotocol() {
	case protoV6, protoV7:
		var jwtErr error
		claimsWSBootstrap, jwtErr = h.getWsBootstrap(requestCtx)
		if jwtErr != nil {
			log.Println("jwt auth failed: " + jwtErr.Error())
			sendAndForget(conn, events.ConnectionRejectedBadWsBootstrapPrepared)
			return
		}
	case protoV8:
		var jwtErr error
		claimsProjectJWT, jwtErr = h.getProjectJWT(requestCtx)
		if jwtErr != nil {
			log.Println("jwt auth failed: " + jwtErr.Error())
			sendAndForget(conn, events.ConnectionRejectedBadWsBootstrapPrepared)
			return
		}
	default:
		log.Println("jwt auth failed: bad proto")
		sendAndForget(conn, events.ConnectionRejectedBadWsBootstrapPrepared)
		return
	}
	bootstrapConnection := conn.Subprotocol() == protoV8

	writerChanges := make(chan bool)
	writeQueue := make(chan types.WriteQueueEntry, 10)
	go func() {
		defer close(writeQueue)
		pendingWriters := 1
		for addWriter := range writerChanges {
			if addWriter {
				pendingWriters++
			} else {
				pendingWriters--
			}
			if pendingWriters == 0 {
				close(writerChanges)
			}
		}
	}()
	defer func() {
		writerChanges <- false
	}()

	ctx, disconnect := context.WithCancel(requestCtx)
	defer disconnect()

	var c *types.Client
	var clientErr error
	if bootstrapConnection {
		c, clientErr = types.NewClient(
			claimsProjectJWT.ProjectId, types.User{
				// User will get populated by realTime.Manager.BootstrapWS.
			},
			writerChanges, writeQueue, disconnect,
		)
	} else {
		c, clientErr = types.NewClient(
			claimsWSBootstrap.ProjectId, claimsWSBootstrap.User,
			writerChanges, writeQueue, disconnect,
		)
		if clientErr != nil {
			log.Println("client setup failed: " + clientErr.Error())
			sendAndForget(conn, events.ConnectionRejectedInternalErrorPrepared)
			return
		}
	}

	if h.rtm.IsShuttingDown() {
		sendAndForget(conn, events.ConnectionRejectedRetryPrepared)
		return
	}

	if !bootstrapConnection {
		setupTime.End()
		if !c.EnsureQueueResponse(events.ConnectionAcceptedResponse(c.PublicId, setupTime)) {
			return
		}
	}

	waitForCtxDone := ctx.Done()

	defer func() {
		disconnect()
		_ = conn.Close()
		_ = h.rtm.Disconnect(c)
	}()
	go func() {
		defer func() {
			disconnect()
			_ = conn.Close()
			for range writeQueue {
				// Flush the queue.
				// Eventually the main goroutine will close the channel.
			}
		}()
		for {
			select {
			case <-waitForCtxDone:
				return
			case entry, ok := <-writeQueue:
				if !ok {
					return
				}
				if entry.Msg != nil {
					err := conn.WritePreparedMessage(entry.Msg)
					if err != nil {
						return
					}
				} else {
					err := conn.WriteMessage(websocket.TextMessage, entry.Blob)
					if err != nil {
						return
					}
				}
				if entry.FatalError {
					return
				}
			}
		}
	}()

	defer func() {
		// Wait for the queue flush.
		// In case queuing from the read-loop failed, this is a noop.
		<-waitForCtxDone
	}()
	if bootstrapConnection {
		bCtx, done := context.WithTimeout(ctx, 10*time.Second)
		blob, err := h.rtm.BootstrapWS(bCtx, c, *claimsProjectJWT)
		done()
		if ctx.Err() != nil {
			return // connection aborted
		}
		if err != nil {
			if errors.IsUnauthorizedError(err) {
				log.Println("jwt auth failed: " + err.Error())
				sendAndForget(conn, events.ConnectionRejectedBadWsBootstrapPrepared)
			} else {
				log.Println("bootstrapWS failed: " + err.Error())
				sendAndForget(conn, events.ConnectionRejectedRetryPrepared)
			}
			return
		}
		setupTime.End()
		r := &types.RPCResponse{
			Body:        blob,
			Callback:    0,
			Name:        "bootstrap",
			Latency:     setupTime,
			ProcessedBy: "self",
		}
		if !c.EnsureQueueResponse(r) {
			return
		}
	}
	for {
		select {
		case <-waitForCtxDone:
			return
		default:
			// Not done yet.
		}
		var request types.RPCRequest
		err := conn.ReadJSON(&request)
		if err != nil {
			c.EnsureQueueMessage(events.BadRequestBulkMessage)
			return
		}
		response := types.RPCResponse{
			Callback: request.Callback,
		}
		tCtx, finishedRPC := context.WithTimeout(ctx, time.Second*10)
		rpc := types.RPC{
			Client:   c,
			Request:  &request,
			Response: &response,
		}
		response.Latency.Begin()
		h.rtm.RPC(tCtx, &rpc)
		finishedRPC()
		if rpc.Response != nil {
			rpc.Response.Latency.End()
			failed := !c.EnsureQueueResponse(&response)
			if failed || rpc.Response.FatalError {
				return
			}
		}
	}
}
