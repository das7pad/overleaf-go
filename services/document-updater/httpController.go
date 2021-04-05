// Golang port of the Overleaf document-updater service
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
)

func newHttpController() httpController {
	return httpController{}
}

type httpController struct {
}

func (h *httpController) GetRouter() http.Handler {
	router := mux.NewRouter()
	router.HandleFunc("/status", h.status)
	projectRouter := router.
		PathPrefix("/project/{projectId}").
		Subrouter()
	projectRouter.Use(validateAndSetId("projectId"))

	docRouter := projectRouter.
		PathPrefix("/doc/{docId}").
		Subrouter()
	docRouter.Use(validateAndSetId("docId"))

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
	_, _ = w.Write([]byte("document-updater is alive (go)\n"))
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

// TODO: move into pkg
type ValidationError string

func (v ValidationError) Error() string {
	return string(v)
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
		if _, is400 := err.(ValidationError); is400 {
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

func (h *httpController) demo(w http.ResponseWriter, r *http.Request) {
	i, err := getNumberFromQuery(r, "foo", 417)
	respond(w, r, int(i), getId(r, "docId"), err, "demo")
}
