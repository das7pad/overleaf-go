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

package router

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/httpUtils"
	"github.com/das7pad/overleaf-go/pkg/jwt/jwtHandler"
	"github.com/das7pad/overleaf-go/pkg/jwt/projectJWT"
	"github.com/das7pad/overleaf-go/pkg/options/jwtOptions"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/events"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/managers/realTime"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/wsServer"
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
			// Validation is performed as part of the bootstrap process.
			nil,
		),
		rateLimitBootstrap: make(chan struct{}, 42),
		writeQueueDepth:    writeQueueDepth,
	}).addRoutes(r)
}

func WS(rtm realTime.Manager, jwtOptionsProject jwtOptions.JWTOptions, writeQueueDepth int) wsServer.Handler {
	h := httpController{
		rtm: rtm,
		u: websocket.Upgrader{
			Subprotocols: []string{
				protoV8,
			},
		},
		jwtProject: projectJWT.New(
			jwtOptionsProject,
			// Validation is performed as part of the bootstrap process.
			nil,
		),
		rateLimitBootstrap: make(chan struct{}, 42),
		writeQueueDepth:    writeQueueDepth,
	}
	return h.wsWsServer
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
	// Avoid overhead from route matching and httpUtils.Context
	router.NewRoute().
		MatcherFunc(func(r *http.Request, _ *mux.RouteMatch) bool {
			return r.Method == http.MethodGet && r.URL.Path == "/socket.io"
		}).
		HandlerFunc(h.wsHTTP)
}

func (h *httpController) getProjectJWT(r *http.Request) (*projectJWT.Claims, error) {
	var blob string
	for _, proto := range websocket.Subprotocols(r) {
		if strings.HasSuffix(proto, protoV8JWTProtoPrefix) {
			blob = proto[:len(proto)-len(protoV8JWTProtoPrefix)]
			break
		}
	}
	if len(blob) == 0 {
		return nil, &errors.ValidationError{Msg: "missing bootstrap blob"}
	}
	return h.jwtProject.Parse([]byte(blob))
}

func sendAndForget(conn *websocket.Conn, entry types.WriteQueueEntry) {
	_ = conn.WritePreparedMessage(entry.Msg)
	_ = conn.Close()
}

func (h *httpController) wsHTTP(w http.ResponseWriter, r *http.Request) {
	t0 := time.Now()

	conn, err := h.u.Upgrade(w, r, nil)
	if err != nil {
		// A 4xx has been generated already.
		return
	}

	if h.rtm.IsShuttingDown() {
		sendAndForget(conn, events.ConnectionRejectedRetryPrepared)
		return
	}

	claimsProjectJWT, err := h.getProjectJWT(r)
	if err != nil {
		log.Println("jwt auth failed: " + err.Error())
		sendAndForget(conn, events.ConnectionRejectedBadWsBootstrapPrepared)
		return
	}
	h.ws(conn, t0, *claimsProjectJWT)
}

func (h *httpController) wsWsServer(c net.Conn, brw *wsServer.RWBuffer, t0 time.Time, parseRequest func(parseJWT func([]byte)) error) error {
	claims := projectJWT.Claims{}
	var jwtError error
	err := parseRequest(func(blob []byte) {
		jwtError = h.jwtProject.ParseInto(&claims, blob)
	})
	if err != nil {
		return err
	}
	buf := brw.WriteBuffer
	conn := websocket.NewConn(c, true, 2048, 2048, nil, brw.Reader, buf)
	if jwtError != nil {
		log.Println("jwt auth failed: " + jwtError.Error())
		sendAndForget(conn, events.ConnectionRejectedBadWsBootstrapPrepared)
		return nil
	}
	h.ws(conn, t0, claims)
	return nil
}

func (h *httpController) ws(conn *websocket.Conn, t0 time.Time, claimsProjectJWT projectJWT.Claims) {
	if h.rtm.IsShuttingDown() {
		sendAndForget(conn, events.ConnectionRejectedRetryPrepared)
		return
	}

	// The request context will get cancelled once the handler returns.
	// Upgrading/hijacking has stopped the reader for detecting request abort.
	ctx, disconnect := context.WithCancel(context.Background())
	writeQueue := make(chan types.WriteQueueEntry, h.writeQueueDepth)
	c := types.NewClient(writeQueue, disconnect)

	go h.writeLoop(ctx, disconnect, conn, writeQueue)

	if !h.bootstrap(t0, c, claimsProjectJWT) {
		h.rtm.Disconnect(c)
		return
	}

	go h.readLoop(ctx, disconnect, conn, c)
}

func (h *httpController) writeLoop(ctx context.Context, disconnect context.CancelFunc, conn *websocket.Conn, writeQueue chan types.WriteQueueEntry) {
	defer func() {
		disconnect()
		_ = conn.Close()
		for range writeQueue {
			// Flush the queue.
			// Eventually the room cleanup will close the channel.
		}
		if c, ok := conn.NetConn().(*wsServer.BufferedConn); ok {
			c.ReleaseBuffers()
		}
	}()
	waitForCtxDone := ctx.Done()
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
				_ = conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
				if err := conn.WritePreparedMessage(entry.Msg); err != nil {
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
				blob, err := entry.RPCResponse.MarshalJSON()
				if err != nil {
					return
				}
				_ = conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
				err = conn.WriteMessage(websocket.TextMessage, blob)
				entry.RPCResponse.ReleaseBuffer(blob)
				if err != nil {
					return
				}
			}
			if entry.FatalError {
				return
			}
		}
	}
}

func (h *httpController) bootstrap(t0 time.Time, c *types.Client, claimsProjectJWT projectJWT.Claims) bool {
	ctx, done := context.WithDeadline(context.Background(), t0.Add(10*time.Second))
	defer done()

	resp := types.RPCResponse{Name: "bootstrap"}
	resp.Latency.SetBegin(t0)

	h.rateLimitBootstrap <- struct{}{}
	err := h.rtm.BootstrapWS(ctx, &resp, c, claimsProjectJWT)
	<-h.rateLimitBootstrap

	if err != nil {
		err = errors.Tag(err, fmt.Sprintf(
			"user=%s project=%s",
			claimsProjectJWT.UserId, claimsProjectJWT.ProjectId,
		))
		if errors.IsUnauthorizedError(err) {
			log.Println("jwt auth failed: " + err.Error())
			c.EnsureQueueMessage(events.ConnectionRejectedBadWsBootstrapPrepared)
		} else {
			log.Println("bootstrapWS failed: " + err.Error())
			c.EnsureQueueMessage(events.ConnectionRejectedRetryPrepared)
		}
		return false
	}
	resp.Latency.End()
	return c.EnsureQueueResponse(&resp)
}

func (h *httpController) readLoop(ctx context.Context, disconnect context.CancelFunc, conn *websocket.Conn, c *types.Client) {
	defer h.rtm.Disconnect(c)
	for {
		if conn.SetReadDeadline(time.Now().Add(idleTime)) != nil {
			disconnect()
			_ = conn.Close()
			return
		}
		var request types.RPCRequest
		if err := conn.ReadJSON(&request); err != nil {
			if shouldTriggerDisconnect(err) {
				disconnect()
				_ = conn.Close()
				return
			}
			c.EnsureQueueMessage(events.BadRequestBulkMessage)
			return
		}
		response := types.RPCResponse{
			Callback: request.Callback,
		}
		t0 := time.Now()
		tCtx, finishedRPC := context.WithDeadline(ctx, t0.Add(time.Second*10))
		rpc := types.RPC{
			Client:   c,
			Request:  &request,
			Response: &response,
		}
		response.Latency.SetBegin(t0)
		h.rtm.RPC(tCtx, &rpc)
		finishedRPC()
		rpc.Response.Latency.End()
		if !c.EnsureQueueResponse(&response) || rpc.Response.FatalError {
			// Do not process further rpc calls after a fatal error.
			return
		}
	}
}

func shouldTriggerDisconnect(err error) bool {
	if _, ok := err.(*websocket.CloseError); ok {
		return true
	}
	if e, ok := err.(net.Error); ok && e.Timeout() {
		return true
	}
	return false
}
