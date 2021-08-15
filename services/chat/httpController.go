// Golang port of the Overleaf chat service
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
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"unicode"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/services/chat/pkg/managers/chat"
)

func newHttpController(cm chat.Manager) httpController {
	return httpController{cm: cm}
}

type httpController struct {
	cm chat.Manager
}

func (h *httpController) GetRouter() http.Handler {
	router := mux.NewRouter()
	router.HandleFunc("/status", h.status)
	projectRouter := router.
		PathPrefix("/project/{projectId}").
		Subrouter()
	projectRouter.Use(validateAndSetId("projectId"))

	threadRouter := projectRouter.
		PathPrefix("/thread/{threadId}").
		Subrouter()
	threadRouter.Use(validateAndSetId("threadId"))

	threadMessagesRouter := threadRouter.
		PathPrefix("/messages/{messageId}").
		Subrouter()
	threadMessagesRouter.Use(validateAndSetId("messageId"))

	projectRouter.
		NewRoute().
		Methods(http.MethodGet).
		Path("/messages").
		HandlerFunc(h.getGlobalMessages)
	projectRouter.
		NewRoute().
		Methods(http.MethodPost).
		Path("/messages").
		HandlerFunc(h.sendGlobalMessages)
	projectRouter.
		NewRoute().
		Methods(http.MethodGet).
		Path("/threads").
		HandlerFunc(h.getAllThreads)

	threadRouter.
		NewRoute().
		Methods(http.MethodPost).
		Path("/messages").
		HandlerFunc(h.sendThreadMessage)
	threadRouter.
		NewRoute().
		Methods(http.MethodPost).
		Path("/resolve").
		HandlerFunc(h.resolveThread)
	threadRouter.
		NewRoute().
		Methods(http.MethodPost).
		Path("/reopen").
		HandlerFunc(h.reopenThread)
	threadRouter.
		NewRoute().
		Methods(http.MethodDelete).
		Path("").
		HandlerFunc(h.deleteThread)

	threadMessagesRouter.
		NewRoute().
		Methods(http.MethodPost).
		Path("/edit").
		HandlerFunc(h.editMessage)
	threadMessagesRouter.
		NewRoute().
		Methods(http.MethodDelete).
		Path("").
		HandlerFunc(h.deleteMessage)

	return router
}

func validateAndSetId(name string) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, err := primitive.ObjectIDFromHex(mux.Vars(r)[name])
			if err != nil || id == primitive.NilObjectID {
				errorResponse(w, 400, "invalid "+name)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func getId(r *http.Request, name string) primitive.ObjectID {
	id, err := primitive.ObjectIDFromHex(mux.Vars(r)[name])
	if err != nil {
		// The validation middleware should have blocked this request.
		log.Printf(
			"%s not validated on route %s %s",
			name, r.Method, r.URL.Path,
		)
		panic(err)
	}
	return id
}

func errorResponse(w http.ResponseWriter, code int, message string) {
	w.WriteHeader(code)

	// Align the error messages with the NodeJS implementation/tests.
	if message == "invalid payload" {
		// Realistically only the user_id field is of interest.
		message = "invalid user_id"
	}
	// Emit a capitalized error message.
	message = fmt.Sprintf(
		"%s%s",
		string(unicode.ToTitle(rune(message[0]))),
		message[1:],
	)
	// Report errors is route parameter validation as projectId -> project_id.
	message = strings.ReplaceAll(message, "Id", "_id")

	// Flush it and ignore any errors.
	_, _ = w.Write([]byte(message))
}

func (h *httpController) status(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(200)
	_, _ = w.Write([]byte("chat is alive (go)\n"))
}

func getNumberFromQuery(
	r *http.Request,
	key string,
	fallback float64,
) (float64, error) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return fallback, nil
	}
	return strconv.ParseFloat(raw, 64)
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
			errorResponse(w, 400, err.Error())
			return
		}
		log.Printf("%s %s: %s: %v", r.Method, r.URL.Path, msg, err)
		errorResponse(w, 500, msg)
		return
	}
	w.WriteHeader(code)
	if body != nil {
		_ = json.NewEncoder(w).Encode(body)
	}
}

const DefaultMessageLimit = 50

func (h *httpController) getGlobalMessages(w http.ResponseWriter, r *http.Request) {
	limit, err := getNumberFromQuery(r, "limit", DefaultMessageLimit)
	if err != nil {
		errorResponse(w, 400, "invalid limit parameter")
		return
	}
	before, err := getNumberFromQuery(r, "before", 0)
	if err != nil {
		errorResponse(w, 400, "invalid before parameter")
		return
	}

	messages, err := h.cm.GetGlobalMessages(
		r.Context(),
		getId(r, "projectId"),
		int64(limit),
		before,
	)
	respond(w, r, 200, messages, err, "cannot get global messages")
}

type sendMessageRequestBody struct {
	Content string             `json:"content"`
	UserId  primitive.ObjectID `json:"user_id"`
}

func parseSendMessageRequest(
	r *http.Request,
) (string, primitive.ObjectID, error) {
	var requestBody sendMessageRequestBody
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		return "", primitive.NilObjectID, &errors.ValidationError{
			Msg: "invalid payload",
		}
	}
	if requestBody.UserId == primitive.NilObjectID {
		return "", primitive.NilObjectID, &errors.ValidationError{
			Msg: "invalid user_id",
		}
	}
	return requestBody.Content, requestBody.UserId, nil
}

func (h *httpController) sendGlobalMessages(w http.ResponseWriter, r *http.Request) {
	content, userId, validationError := parseSendMessageRequest(r)
	if validationError != nil {
		errorResponse(w, 400, validationError.Error())
		return
	}
	message, err := h.cm.SendGlobalMessage(
		r.Context(),
		getId(r, "projectId"),
		content,
		userId,
	)
	respond(w, r, 201, message, err, "cannot send global message")
}

func (h *httpController) getAllThreads(w http.ResponseWriter, r *http.Request) {
	threads, err := h.cm.GetAllThreads(
		r.Context(),
		getId(r, "projectId"),
	)
	respond(w, r, 200, threads, err, "cannot get all threads")
}

func (h *httpController) sendThreadMessage(w http.ResponseWriter, r *http.Request) {
	content, userId, validationError := parseSendMessageRequest(r)
	if validationError != nil {
		errorResponse(w, 400, validationError.Error())
		return
	}
	message, err := h.cm.SendThreadMessage(
		r.Context(),
		getId(r, "projectId"),
		getId(r, "threadId"),
		content,
		userId,
	)
	respond(w, r, 201, message, err, "cannot send thread message")
}

type resolveThreadRequestBody struct {
	UserId primitive.ObjectID `json:"user_id"`
}

func (h *httpController) resolveThread(w http.ResponseWriter, r *http.Request) {
	var requestBody resolveThreadRequestBody
	err := json.NewDecoder(r.Body).Decode(&requestBody)
	if err != nil || requestBody.UserId == primitive.NilObjectID {
		errorResponse(w, 400, "invalid user_id")
		return
	}
	err = h.cm.ResolveThread(
		r.Context(),
		getId(r, "projectId"),
		getId(r, "threadId"),
		requestBody.UserId,
	)
	respond(w, r, 204, nil, err, "cannot resolve thread")
}

func (h *httpController) reopenThread(w http.ResponseWriter, r *http.Request) {
	err := h.cm.ReopenThread(
		r.Context(),
		getId(r, "projectId"),
		getId(r, "threadId"),
	)
	respond(w, r, 204, nil, err, "cannot reopen thread")
}

func (h *httpController) deleteThread(w http.ResponseWriter, r *http.Request) {
	err := h.cm.DeleteThread(
		r.Context(),
		getId(r, "projectId"),
		getId(r, "threadId"),
	)
	respond(w, r, 204, nil, err, "cannot delete thread")
}

type editMessageRequestBody struct {
	Content string `json:"content"`
}

func (h *httpController) editMessage(w http.ResponseWriter, r *http.Request) {
	var requestBody editMessageRequestBody
	err := json.NewDecoder(r.Body).Decode(&requestBody)
	if err != nil {
		errorResponse(w, 400, "invalid request")
		return
	}
	err = h.cm.EditMessage(
		r.Context(),
		getId(r, "projectId"),
		getId(r, "threadId"),
		getId(r, "messageId"),
		requestBody.Content,
	)
	respond(w, r, 204, nil, err, "cannot edit message")
}

func (h *httpController) deleteMessage(w http.ResponseWriter, r *http.Request) {
	err := h.cm.DeleteMessage(
		r.Context(),
		getId(r, "projectId"),
		getId(r, "threadId"),
		getId(r, "messageId"),
	)
	respond(w, r, 204, nil, err, "cannot delete message")
}
