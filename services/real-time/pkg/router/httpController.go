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
	"time"

	"github.com/gorilla/websocket"

	"github.com/das7pad/overleaf-go/pkg/httpUtils"
	"github.com/das7pad/overleaf-go/pkg/jwt/wsBootstrap"
	"github.com/das7pad/overleaf-go/pkg/options/jwtOptions"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/events"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/managers/realTime"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

func New(rtm realTime.Manager, jwtOptions jwtOptions.JWTOptions) *httpUtils.Router {
	r := httpUtils.NewRouter(&httpUtils.RouterOptions{
		Ready: func() bool {
			return !rtm.IsShuttingDown()
		},
	})
	Add(r, rtm, jwtOptions)
	return r
}

func Add(r *httpUtils.Router, rtm realTime.Manager, jwtOptions jwtOptions.JWTOptions) {
	(&httpController{
		rtm: rtm,
		u: websocket.Upgrader{
			Subprotocols: []string{"v6.real-time.overleaf.com"},
		},
		jwt: httpUtils.NewJWTHandlerFromQuery(
			wsBootstrap.New(jwtOptions), jwtQueryParameter,
		),
	}).addRoutes(r)
}

type httpController struct {
	rtm realTime.Manager
	u   websocket.Upgrader
	jwt *httpUtils.JWTHTTPHandler
}

const jwtQueryParameter = "bootstrap"

func (h *httpController) addRoutes(router *httpUtils.Router) {
	router.GET("/socket.io", h.ws)
}

func (h *httpController) getWsBootstrap(c *httpUtils.Context) (*wsBootstrap.Claims, error) {
	genericClaims, jwtError := h.jwt.Parse(c)
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

	claims, jwtErr := h.getWsBootstrap(requestCtx)
	if jwtErr != nil {
		log.Println("jwt auth failed: " + jwtErr.Error())
		sendAndForget(conn, events.ConnectionRejectedBadWsBootstrapPrepared)
		return
	}

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

	ctx, cancel := context.WithCancel(requestCtx)
	defer cancel()

	c, clientErr := types.NewClient(
		claims.ProjectId, claims.User,
		writerChanges, writeQueue, cancel,
	)
	if clientErr != nil {
		log.Println("client setup failed: " + clientErr.Error())
		sendAndForget(conn, events.ConnectionRejectedInternalErrorPrepared)
		return
	}

	if h.rtm.IsShuttingDown() {
		sendAndForget(conn, events.ConnectionRejectedRetryPrepared)
		return
	}

	setupTime.End()
	if !c.EnsureQueueResponse(events.ConnectionAcceptedResponse(c.PublicId, setupTime)) {
		return
	}

	waitForCtxDone := ctx.Done()

	defer func() {
		cancel()
		_ = conn.Close()
		_ = h.rtm.Disconnect(c)
	}()
	go func() {
		defer func() {
			cancel()
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
			Context:  tCtx,
			Client:   c,
			Request:  &request,
			Response: &response,
		}
		response.Latency.Begin()
		h.rtm.RPC(&rpc)
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