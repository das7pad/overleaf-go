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

package router

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/httpUtils"
	"github.com/das7pad/overleaf-go/pkg/jwt/jwtHandler"
	"github.com/das7pad/overleaf-go/pkg/jwt/projectJWT"
	"github.com/das7pad/overleaf-go/pkg/options/jwtOptions"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/events"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/managers/realTime"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

func New(rtm realTime.Manager, jwtOptionsProject jwtOptions.JWTOptions, writeQueueDepth int) *httpUtils.Router {
	r := httpUtils.NewRouter(&httpUtils.RouterOptions{
		Ready: func() bool {
			return !rtm.IsShuttingDown()
		},
	})
	Add(r, rtm, jwtOptionsProject, writeQueueDepth)
	return r
}

func Add(r *httpUtils.Router, rtm realTime.Manager, jwtOptionsProject jwtOptions.JWTOptions, writeQueueDepth int) {
	(&httpController{
		rtm: rtm,
		u: websocket.Upgrader{
			Subprotocols: []string{
				protoV8,
			},
		},
		jwtProject: projectJWT.New(
			jwtOptionsProject,
			func(ctx context.Context, projectId, userId sharedTypes.UUID, projectEpoch, userEpoch int64) error {
				// validation is performed as part of bootstrap
				return &errors.NotAuthorizedError{}
			},
		),
		rateLimitBootstrap: make(chan struct{}, 42),
		writeQueueDepth:    writeQueueDepth,
	}).addRoutes(r)
}

type httpController struct {
	rtm                realTime.Manager
	u                  websocket.Upgrader
	jwtProject         jwtHandler.JWTHandler[*projectJWT.Claims]
	rateLimitBootstrap chan struct{}
	writeQueueDepth    int
}

const (
	protoV8               = "v8.real-time.overleaf.com"
	protoV8JWTProtoPrefix = ".bootstrap.v8.real-time.overleaf.com"

	// two skipped heath checks plus latency
	idleTime = time.Minute + 10*time.Second
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
	return h.jwtProject.Parse(blob)
}

func sendAndForget(conn *websocket.Conn, entry types.WriteQueueEntry) {
	_ = conn.WritePreparedMessage(entry.Msg)
}

func (h *httpController) ws(requestCtx *httpUtils.Context) {
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

	claimsProjectJWT, jwtErr := h.getProjectJWT(requestCtx)
	if jwtErr != nil {
		log.Println("jwt auth failed: " + jwtErr.Error())
		sendAndForget(conn, events.ConnectionRejectedBadWsBootstrapPrepared)
		return
	}

	writeQueue := make(chan types.WriteQueueEntry, h.writeQueueDepth)

	// Upgrading/hijacking has stopped the reader for detecting request abort.
	ctx, disconnect := context.WithCancel(context.Background())
	defer disconnect()

	c, clientErr := types.NewClient(writeQueue, disconnect)
	if clientErr != nil {
		log.Println("client setup failed: " + clientErr.Error())
		sendAndForget(conn, events.ConnectionRejectedInternalErrorPrepared)
		return
	}

	if h.rtm.IsShuttingDown() {
		sendAndForget(conn, events.ConnectionRejectedRetryPrepared)
		return
	}

	waitForCtxDone := ctx.Done()

	defer func() {
		disconnect()
		_ = conn.Close()
		h.rtm.Disconnect(c)
	}()
	go func() {
		defer func() {
			disconnect()
			_ = conn.Close()
			for range writeQueue {
				// Flush the queue.
				// Eventually the room cleanup will close the channel.
			}
		}()
		var lsr []types.LazySuccessResponse
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
				} else if len(lsr) < 15 &&
					entry.RPCResponse.IsLazySuccessResponse() {
					lsr = append(lsr, types.LazySuccessResponse{
						Callback: entry.RPCResponse.Callback,
						Latency:  entry.RPCResponse.Latency,
					})
				} else {
					if len(lsr) > 0 {
						entry.RPCResponse.LazySuccessResponses = lsr
						lsr = lsr[:0]
					}
					blob, err := json.Marshal(entry.RPCResponse)
					if err != nil {
						return
					}
					err = conn.WriteMessage(websocket.TextMessage, blob)
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
	{
		bCtx, done := context.WithTimeout(ctx, 10*time.Second)
		h.rateLimitBootstrap <- struct{}{}
		blob, err := h.rtm.BootstrapWS(bCtx, c, *claimsProjectJWT)
		<-h.rateLimitBootstrap
		done()
		if ctx.Err() != nil {
			return // connection aborted
		}
		if err != nil {
			err = errors.Tag(err, fmt.Sprintf(
				"user=%s project=%s",
				claimsProjectJWT.UserId, claimsProjectJWT.ProjectId,
			))
			if errors.IsUnauthorizedError(err) {
				log.Println("jwt auth failed: " + err.Error())
				sendAndForget(conn, events.ConnectionRejectedBadWsBootstrapPrepared)
			} else {
				log.Println("bootstrapWS failed: " + err.Error())
				sendAndForget(conn, events.ConnectionRejectedRetryPrepared)
			}
			return
		}
		setupTime := sharedTypes.Timed{}
		setupTime.SetBegin(requestCtx.T0())
		setupTime.End()
		r := types.RPCResponse{
			Body:    blob,
			Name:    "bootstrap",
			Latency: setupTime,
		}
		if !c.EnsureQueueResponse(&r) {
			return
		}
	}
	for {
		if conn.SetReadDeadline(time.Now().Add(idleTime)) != nil {
			disconnect()
			return
		}
		var request types.RPCRequest
		if err := conn.ReadJSON(&request); err != nil {
			if _, ok := err.(*websocket.CloseError); ok {
				disconnect()
				return
			}
			if e, ok := err.(net.Error); ok && e.Timeout() {
				disconnect()
				return
			}
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
		rpc.Response.Latency.End()
		if !c.EnsureQueueResponse(&response) || rpc.Response.FatalError {
			// Do not process further rpc calls after a fatal error.
			return
		}
	}
}
