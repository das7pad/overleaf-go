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
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/document-updater/pkg/errors"
	"github.com/das7pad/document-updater/pkg/managers/documentUpdater"
	"github.com/das7pad/document-updater/pkg/types"
)

func newHttpController(dum documentUpdater.Manager) httpController {
	return httpController{
		dum: dum,
	}
}

type httpController struct {
	dum documentUpdater.Manager
}

func (h *httpController) GetRouter() http.Handler {
	router := mux.NewRouter()
	router.NotFoundHandler = http.HandlerFunc(h.handle404)
	router.HandleFunc("/status", h.status)

	projectRouter := router.
		PathPrefix("/project/{projectId}").
		Subrouter()
	projectRouter.Use(validateAndSetId("projectId"))

	projectRouter.
		NewRoute().
		Methods(http.MethodDelete).
		Path("").
		HandlerFunc(h.flushAndDeleteProject)
	projectRouter.
		NewRoute().
		Methods(http.MethodPost).
		Path("/clearState").
		HandlerFunc(h.clearProjectState)
	projectRouter.
		NewRoute().
		Methods(http.MethodPost).
		Path("/flush").
		HandlerFunc(h.flushProject)
	projectRouter.
		NewRoute().
		Methods(http.MethodPost).
		Path("/get_and_flush_if_old").
		HandlerFunc(h.getAndFlushIfOld)

	docRouter := projectRouter.
		PathPrefix("/doc/{docId}").
		Subrouter()
	docRouter.Use(validateAndSetId("docId"))

	docRouter.
		NewRoute().
		Methods(http.MethodGet).
		Path("").
		HandlerFunc(h.getDoc)
	docRouter.
		NewRoute().
		Methods(http.MethodDelete).
		Path("").
		HandlerFunc(h.flushAndDeleteDoc)
	docRouter.
		NewRoute().
		Methods(http.MethodPost).
		Path("/flush").
		HandlerFunc(h.flushDoc)
	docRouter.
		NewRoute().
		Methods(http.MethodGet, http.MethodHead).
		Path("/exists").
		HandlerFunc(h.checkDocExists)

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

	// Flush it and ignore any errors.
	_, _ = w.Write([]byte(message))
}

func (h *httpController) status(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(200)
	_, _ = w.Write([]byte("document-updater is alive (go)\n"))
}

func getVersionFromQuery(
	r *http.Request,
	key string,
	fallback types.Version,
) (types.Version, error) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return fallback, nil
	}
	i, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, err
	}
	return types.Version(i), nil
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
		if errors.IsNotAuthorizedError(err) {
			errorResponse(w, http.StatusForbidden, err.Error())
			return
		}
		if errors.IsNotFoundError(err) {
			errorResponse(w, http.StatusNotFound, err.Error())
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

func (h *httpController) handle404(w http.ResponseWriter, r *http.Request) {
	respond(w, r, 404, nil, errors.New("404"), "404")
}

func (h *httpController) checkDocExists(w http.ResponseWriter, r *http.Request) {
	err := h.dum.CheckDocExists(
		r.Context(),
		getId(r, "projectId"),
		getId(r, "docId"),
	)
	respond(w, r, http.StatusNoContent, nil, err, "cannot check doc exists")
}

func (h *httpController) getDoc(w http.ResponseWriter, r *http.Request) {
	fromVersion, err := getVersionFromQuery(r, "fromVersion", -1)
	if err != nil {
		errorResponse(w, 400, "invalid fromVersion")
		return
	}
	doc, err := h.dum.GetDoc(
		r.Context(),
		getId(r, "projectId"),
		getId(r, "docId"),
		fromVersion,
	)
	respond(w, r, http.StatusOK, doc, err, "cannot get doc")
}

func (h *httpController) flushProject(w http.ResponseWriter, r *http.Request) {
	err := h.dum.FlushProject(
		r.Context(),
		getId(r, "projectId"),
	)
	respond(w, r, http.StatusNoContent, nil, err, "cannot flush project")
}

func (h *httpController) flushDoc(w http.ResponseWriter, r *http.Request) {
	err := h.dum.FlushDoc(
		r.Context(),
		getId(r, "projectId"),
		getId(r, "docId"),
	)
	respond(w, r, http.StatusNoContent, nil, err, "cannot flush doc")
}

func (h *httpController) flushAndDeleteDoc(w http.ResponseWriter, r *http.Request) {
	err := h.dum.FlushAndDeleteDoc(
		r.Context(),
		getId(r, "projectId"),
		getId(r, "docId"),
	)
	respond(w, r, http.StatusNoContent, nil, err, "cannot flush and delete doc")
}

func (h *httpController) flushAndDeleteProject(w http.ResponseWriter, r *http.Request) {
	err := h.dum.FlushAndDeleteProject(
		r.Context(),
		getId(r, "projectId"),
	)
	respond(w, r, http.StatusNoContent, nil, err, "cannot flush and delete project")
}

func (h *httpController) getAndFlushIfOld(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	docs, err := h.dum.GetProjectDocsAndFlushIfOld(
		r.Context(),
		getId(r, "projectId"),
		state,
	)
	respond(w, r, http.StatusOK, docs, err, "cannot get and flush old")
}

func (h *httpController) clearProjectState(w http.ResponseWriter, r *http.Request) {
	err := h.dum.ClearProjectState(
		r.Context(),
		getId(r, "projectId"),
	)
	respond(w, r, http.StatusNoContent, nil, err, "cannot clear state")
}
