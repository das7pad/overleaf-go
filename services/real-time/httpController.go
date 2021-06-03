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

	jwtMiddleware "github.com/auth0/go-jwt-middleware"
	"github.com/form3tech-oss/jwt-go"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/real-time/pkg/errors"
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
			Subprotocols: []string{"v5.realTime.overleaf.com"},
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

func (h *httpController) getUserFromJWT(r *http.Request) (*types.User, error) {
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
	if rawUser == nil {
		return nil, &errors.ValidationError{Msg: "corrupt jwt: missing user"}
	}
	user, ok := rawUser.(*types.User)
	if !ok {
		return nil, &errors.ValidationError{Msg: "corrupt jwt: malformed user"}
	}
	return user, nil
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

func (h *httpController) ws(w http.ResponseWriter, r *http.Request) {
	u, upgradeErr := h.u.Upgrade(w, r, nil)
	if upgradeErr != nil {
		// A 4xx has been generated already.
		return
	}

	user, jwtErr := h.getUserFromJWT(r)
	if jwtErr != nil {
		msg := "cannot read user from jwt"
		if errors.IsValidationError(jwtErr) {
			msg = jwtErr.Error()
		}
		_ = u.WriteJSON(&types.RPCResponse{Error: msg})
		_ = u.Close()
		return
	}

	c := types.Client{
		PublicId:   "..",
		User:       user,
		WriteQueue: make(chan *types.RPCResponse, 10),
	}

	ctx, cancel := context.WithCancel(r.Context())

	defer func() {
		cancel()
		close(c.WriteQueue)
		for range c.WriteQueue {
			// Flush queue.
		}
		_ = u.Close()
	}()
	go func() {
		waitForCtxDone := ctx.Done()
		for {
			select {
			case <-waitForCtxDone:
				return
			case response, ok := <-c.WriteQueue:
				if !ok {
					return
				}
				if err := u.WriteJSON(&response); err != nil {
					break
				}
			}
		}
	}()

	var request types.RPCRequest
	for {
		err := u.ReadJSON(&request)
		if err != nil {
			break
		}
		response := types.RPCResponse{
			Callback: request.Callback,
		}
		err = h.rtm.RPC(ctx, &c, &request, &response)
		if err != nil {
			break
		}
		c.WriteQueue <- &response
	}
}
