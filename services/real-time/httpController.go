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

package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	jwtMiddleware "github.com/auth0/go-jwt-middleware"
	"github.com/form3tech-oss/jwt-go"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/real-time/pkg/errors"
	"github.com/das7pad/real-time/pkg/events"
	"github.com/das7pad/real-time/pkg/managers/realTime"
	"github.com/das7pad/real-time/pkg/types"
)

func newHttpController(rtm realTime.Manager, jwtOptions jwtMiddleware.Options) httpController {
	jwtOptions.Extractor = jwtMiddleware.FromParameter(jwtQueryParameter)
	jwtOptions.UserProperty = jwtQueryParameter
	jwtOptions.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err string) {
		// noop. We are handling the error after the ws upgrade.
	}
	return httpController{
		rtm: rtm,
		u: websocket.Upgrader{
			Subprotocols: []string{"v5.real-time.overleaf.com"},
		},
		jwt: jwtMiddleware.New(jwtOptions),
	}
}

type httpController struct {
	rtm realTime.Manager
	u   websocket.Upgrader
	jwt *jwtMiddleware.JWTMiddleware
}

const jwtQueryParameter = "bootstrap"

func (h *httpController) GetRouter() http.Handler {
	router := mux.NewRouter()
	router.HandleFunc("/status", h.status)

	projectRouter := router.
		PathPrefix("/project/{projectId}").
		Subrouter()
	projectRouter.Use(validateAndSetId("projectId"))

	userRouter := projectRouter.
		PathPrefix("/user/{userId}").
		Subrouter()
	userRouter.Use(validateAndSetId("userId"))

	router.HandleFunc("/socket.io", h.ws)
	router.HandleFunc("/socket.io/socket.io.js", h.clientBlob)
	return router
}

func validateAndSetId(name string) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, err := primitive.ObjectIDFromHex(getRawIdFromRequest(r, name))
			if err != nil || id == primitive.NilObjectID {
				errorResponse(w, http.StatusBadRequest, "invalid "+name)
				return
			}
			ctx := context.WithValue(r.Context(), name, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

const (
	emailField     = "email"
	firstNameField = "firstName"
	idField        = "user_id"
	lastNameField  = "lastName"
)

var (
	userFields = []string{
		emailField,
		firstNameField,
		idField,
		lastNameField,
	}
)

func (h *httpController) getWsBootstrap(r *http.Request) (*types.WsBootstrap, error) {
	if err := h.jwt.CheckJWT(nil, r); err != nil {
		return nil, err
	}

	rawToken := r.Context().Value(jwtQueryParameter)
	if rawToken == nil {
		return nil, &errors.ValidationError{Msg: "missing jwt"}
	}
	token, ok := rawToken.(*jwt.Token)
	if !ok {
		return nil, &errors.ValidationError{Msg: "malformed jwt"}
	}
	claims := token.Claims.(jwt.MapClaims)
	rawUser := claims["user"]
	user := &types.User{}
	if rawUser == nil {
		user.Id = primitive.NilObjectID
	} else {
		userDetails, ok2 := rawUser.(map[string]interface{})
		if !ok2 {
			return nil, &errors.ValidationError{
				Msg: "corrupt jwt: malformed user",
			}
		}
		for _, f := range userFields {
			if userDetails[f] == nil {
				return nil, &errors.ValidationError{
					Msg: "corrupt jwt: malformed user." + f,
				}
			}
			s, ok3 := userDetails[f].(string)
			if !ok3 {

			}
			switch f {
			case emailField:
				user.Email = s
			case firstNameField:
				user.FirstName = s
			case idField:
				userId, err := primitive.ObjectIDFromHex(s)
				if err != nil {
					return nil, &errors.ValidationError{
						Msg: "corrupt jwt: malformed user.id",
					}
				}
				user.Id = userId
			case lastNameField:
				user.LastName = s
			}
		}
	}
	rawProjectId := claims["projectId"]
	if rawProjectId == nil {
		return nil, &errors.ValidationError{
			Msg: "corrupt jwt: missing projectId",
		}
	}
	projectId, err := primitive.ObjectIDFromHex(rawProjectId.(string))
	if err != nil {
		return nil, &errors.ValidationError{
			Msg: "corrupt jwt: malformed projectId",
		}
	}
	return &types.WsBootstrap{
		ProjectId: projectId, User: user,
	}, nil
}

func getParam(r *http.Request, name string) string {
	return mux.Vars(r)[name]
}
func getRawIdFromRequest(r *http.Request, name string) string {
	return getParam(r, name)
}

func errorResponse(w http.ResponseWriter, code int, message string) {
	w.WriteHeader(code)

	// Flush it and ignore any errors.
	_, _ = w.Write([]byte(message))
}

func respond(
	w http.ResponseWriter,
	r *http.Request,
	code int,
	body interface{},
	err error,
	msg string,
) {
	if err != nil {
		if errors.IsValidationError(err) {
			errorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.IsInvalidState(err) {
			errorResponse(w, http.StatusConflict, err.Error())
			return
		}
		log.Printf("%s %s: %s: %s", r.Method, r.URL.Path, msg, err)
		errorResponse(w, http.StatusInternalServerError, msg)
		return
	}
	if body == nil {
		w.WriteHeader(code)
	} else {
		w.Header().Set(
			"Content-Type",
			"application/json; charset=utf-8",
		)
		if code != http.StatusOK {
			w.WriteHeader(code)
		}
		_ = json.NewEncoder(w).Encode(body)
	}
}

func (h *httpController) status(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("real-time is alive (go)\n"))
}

func (h *httpController) clientBlob(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	w.WriteHeader(200)
	_, _ = w.Write([]byte("window.io='plain'"))
}

func (h *httpController) ws(w http.ResponseWriter, r *http.Request) {
	conn, upgradeErr := h.u.Upgrade(w, r, nil)
	if upgradeErr != nil {
		// A 4xx has been generated already.
		return
	}
	defer func() { _ = conn.Close() }()

	wsBootstrap, jwtErr := h.getWsBootstrap(r)
	if jwtErr != nil {
		log.Println("jwt auth failed: " + jwtErr.Error())
		_ = conn.WritePreparedMessage(
			events.ConnectionRejectedBadWsBootstrapPrepared.Msg,
		)
		return
	}

	writeQueue := make(chan *types.WriteQueueEntry, 10)
	defer func() {
		// TODO: rework closing to go through ack from project+doc room.
		time.Sleep(10 * time.Second)
		close(writeQueue)
	}()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	c, clientErr := types.NewClient(wsBootstrap, writeQueue, cancel)
	if clientErr != nil {
		log.Println("client setup failed: " + clientErr.Error())
		_ = conn.WritePreparedMessage(
			events.ConnectionRejectedInternalErrorPrepared.Msg,
		)
		return
	}

	if c.QueueResponse(events.ConnectionAcceptedResponse(c.PublicId)) != nil {
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
		h.rtm.RPC(&rpc)
		if rpc.Response != nil {
			failed := !c.EnsureQueueResponse(&response)
			if failed || rpc.Response.FatalError {
				return
			}
		}
	}
}
