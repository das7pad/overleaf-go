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
	"time"

	"github.com/gorilla/mux"
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
	h := httpController{
		rtm: rtm,
		jwtProject: projectJWT.New(
			jwtOptionsProject,
			// Validation is performed as part of the bootstrap process.
			nil,
		),
		bootstrapQueue:  make(chan bootstrapWSDetails, 120),
		writeQueueDepth: writeQueueDepth,
	}
	h.startWorker()
	h.addRoutes(r)
}

func WS(rtm realTime.Manager, jwtOptionsProject jwtOptions.JWTOptions, writeQueueDepth int) *WSServer {
	h := httpController{
		rtm: rtm,
		jwtProject: projectJWT.New(
			jwtOptionsProject,
			// Validation is performed as part of the bootstrap process.
			nil,
		),
		bootstrapQueue:  make(chan bootstrapWSDetails, 120),
		writeQueueDepth: writeQueueDepth,
	}
	h.startWorker()
	srv := WSServer{h: &h}
	srv.ok.Store(true)
	return &srv
}

type httpController struct {
	rtm             realTime.Manager
	jwtProject      *jwtHandler.JWTHandler[*projectJWT.Claims]
	bootstrapQueue  chan bootstrapWSDetails
	writeQueueDepth int
}

func (h *httpController) startWorker() {
	for i := 0; i < 60; i++ {
		go h.bootstrapWorker()
	}
}

const (
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

func sendAndForget(conn *websocket.Conn, entry types.WriteQueueEntry) {
	_ = conn.WritePreparedMessage(entry.Msg)
	_ = conn.Close()
}

func (h *httpController) wsHTTP(w http.ResponseWriter, r *http.Request) {
	t0 := time.Now()
	claims := projectJWT.Claims{}
	var jwtError error
	c, brw, err := HTTPUpgrade(w, r, func(blob []byte) {
		jwtError = h.jwtProject.ParseInto(&claims, blob, t0)
	})
	if err != nil {
		// A 4xx has been generated already.
		return
	}

	buf := brw.AvailableBuffer()[0:4096]
	conn := websocket.NewConn(c, true, 4096, 4096, nil, brw.Reader, buf)
	if jwtError != nil {
		log.Println("jwt auth failed: " + jwtError.Error())
		sendAndForget(conn, events.ConnectionRejectedBadWsBootstrapPrepared)
		return
	}
	h.ws(conn, t0, claims)
}

func (h *httpController) wsWsServer(c *wsConn, t0 time.Time) error {
	claims := projectJWT.Claims{}
	var jwtError error
	err := c.parseWsRequest(func(blob []byte) {
		jwtError = h.jwtProject.ParseInto(&claims, blob, t0)
	})
	if err != nil {
		return err
	}
	buf := c.brw.WriteBuffer
	conn := websocket.NewConn(c.BufferedConn, true, 2048, 2048, nil, c.brw.Reader, buf)
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

	writeQueue := make(chan types.WriteQueueEntry, h.writeQueueDepth)
	c := types.NewClient(writeQueue)

	go h.writeLoop(conn, writeQueue)

	if !h.bootstrap(t0, c, claimsProjectJWT) {
		h.rtm.Disconnect(c)
		return
	}

	go h.readLoop(conn, c)
}

func (h *httpController) writeLoop(conn *websocket.Conn, writeQueue chan types.WriteQueueEntry) {
	defer func() {
		_ = conn.Close()
		for range writeQueue {
			// Flush the queue.
			// Eventually the room cleanup will close the channel.
		}
		if c, ok := conn.NetConn().(*BufferedConn); ok {
			c.ReleaseBuffers()
		}
	}()
	var lsr []types.LazySuccessResponse
	for {
		entry, ok := <-writeQueue
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
			entry.RPCResponse.ReleaseBuffer()
			if err != nil {
				return
			}
		}
		if entry.FatalError {
			return
		}
	}
}

type bootstrapWSDetails struct {
	t0     time.Time
	claims projectJWT.Claims
	resp   *types.RPCResponse
	client *types.Client
	done   chan error
}

func (h *httpController) bootstrapWorker() {
	ctx, done := context.WithCancel(context.Background())
	dl := time.Now().Add(11 * time.Second)
	t := time.AfterFunc(11*time.Second, done)
	for d := range h.bootstrapQueue {
		if dl.Sub(d.t0) < 10*time.Second {
			dl = d.t0.Add(11 * time.Second)
			if !t.Reset(11 * time.Second) {
				t.Stop()
				done()
				ctx, done = context.WithCancel(context.Background())
				t = time.AfterFunc(11*time.Second, done)
			}
		}
		d.done <- h.rtm.BootstrapWS(ctx, d.resp, d.client, d.claims)
	}
	t.Stop()
	done()
}

func (h *httpController) bootstrap(t0 time.Time, c *types.Client, claimsProjectJWT projectJWT.Claims) bool {
	resp := types.RPCResponse{Name: sharedTypes.Bootstrap}
	resp.Latency.SetBegin(t0)

	done := make(chan error)
	h.bootstrapQueue <- bootstrapWSDetails{
		t0:     t0,
		claims: claimsProjectJWT,
		resp:   &resp,
		client: c,
		done:   done,
	}
	err := <-done
	close(done)

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

func (h *httpController) readLoop(conn *websocket.Conn, c *types.Client) {
	defer h.rtm.Disconnect(c)
	var request types.RPCRequest
	for ok := true; ok; {
		request = types.RPCRequest{}
		if err := conn.ReadJSON(&request); err != nil {
			if shouldTriggerDisconnect(err) {
				_ = conn.Close()
				return
			}
			c.EnsureQueueMessage(events.BadRequestBulkMessage)
			return
		}
		if request.Action == types.Ping {
			if conn.SetReadDeadline(time.Now().Add(idleTime)) != nil {
				_ = conn.Close()
				return
			}
			if request.Callback == 1 {
				ok = c.EnsureQueueMessage(events.IdlePingResponse)
			} else {
				// Other RPCs are pending, flush any lazy success responses
				response := types.RPCResponse{Callback: request.Callback}
				ok = c.EnsureQueueResponse(&response)
			}
		} else {
			t0 := time.Now()
			response := types.RPCResponse{Callback: request.Callback}
			response.Latency.SetBegin(t0)
			rpc := types.RPC{
				Client:   c,
				Request:  &request,
				Response: &response,
			}
			ctx, finishedRPC := context.WithDeadline(
				context.Background(), t0.Add(time.Second*10),
			)
			h.rtm.RPC(ctx, &rpc)
			finishedRPC()
			response.Latency.End()
			ok = c.EnsureQueueResponse(&response) && !response.FatalError
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
