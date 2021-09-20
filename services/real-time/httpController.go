// Golang port of Overleaf
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

package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"

	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/httpUtils"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/events"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/managers/realTime"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

func newHttpController(rtm realTime.Manager, jwtOptions httpUtils.JWTOptions) httpController {
	jwtOptions.FromQuery = jwtQueryParameter
	return httpController{
		rtm: rtm,
		u: websocket.Upgrader{
			Subprotocols: []string{"v5.real-time.overleaf.com"},
		},
		jwt: httpUtils.NewJWTHandler(jwtOptions),
	}
}

type httpController struct {
	rtm realTime.Manager
	u   websocket.Upgrader
	jwt *httpUtils.JWTHandler
}

const jwtQueryParameter = "bootstrap"

func (h *httpController) GetRouter() http.Handler {
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/status", h.status)
	router.HEAD("/status", h.status)

	router.GET("/socket.io", h.ws)
	router.GET("/socket.io/socket.io.js", h.clientBlob)
	return router
}

type WsBootstrapUser struct {
	Id    primitive.ObjectID `json:"user_id"`
	Email string             `json:"email"`
	// TODO: align these with the client tracking fields in v6
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

type WsBootstrapClaims struct {
	*jwt.StandardClaims
	User      WsBootstrapUser    `json:"user"`
	ProjectId primitive.ObjectID `json:"projectId"`
}

func (h *httpController) getWsBootstrap(c *gin.Context) (*types.WsBootstrap, error) {
	genericClaims, jwtError := h.jwt.Parse(c, &WsBootstrapClaims{})
	if jwtError != nil {
		return nil, jwtError
	}
	claims := genericClaims.(*WsBootstrapClaims)
	projectId := claims.ProjectId
	user := &types.User{
		Id:        claims.User.Id,
		FirstName: claims.User.FirstName,
		LastName:  claims.User.LastName,
		Email:     claims.User.Email,
	}
	return &types.WsBootstrap{
		ProjectId: projectId, User: user,
	}, nil
}

func (h *httpController) status(c *gin.Context) {
	c.String(http.StatusOK, "real-time is alive (go)\n")
}

func (h *httpController) clientBlob(c *gin.Context) {
	c.Header("Content-Type", "application/javascript")
	c.String(http.StatusOK, "window.io='plain'")
}

func (h *httpController) ws(requestCtx *gin.Context) {
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

	wsBootstrap, jwtErr := h.getWsBootstrap(requestCtx)
	if jwtErr != nil {
		log.Println("jwt auth failed: " + jwtErr.Error())
		_ = conn.WritePreparedMessage(
			events.ConnectionRejectedBadWsBootstrapPrepared.Msg,
		)
		return
	}

	writerChanges := make(chan bool)
	writeQueue := make(chan *types.WriteQueueEntry, 10)
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

	c, clientErr := types.NewClient(wsBootstrap, writerChanges, writeQueue, cancel)
	if clientErr != nil {
		log.Println("client setup failed: " + clientErr.Error())
		_ = conn.WritePreparedMessage(
			events.ConnectionRejectedInternalErrorPrepared.Msg,
		)
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
		rpc := types.RPC{
			Context:  ctx,
			Client:   c,
			Request:  &request,
			Response: &response,
		}
		response.Latency.Begin()
		h.rtm.RPC(&rpc)
		if rpc.Response != nil {
			rpc.Response.Latency.End()
			failed := !c.EnsureQueueResponse(&response)
			if failed || rpc.Response.FatalError {
				return
			}
		}
	}
}
