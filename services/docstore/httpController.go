// Golang port of the Overleaf docstore service
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
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/services/docstore/pkg/errors"
	"github.com/das7pad/overleaf-go/services/docstore/pkg/managers/docstore"
	"github.com/das7pad/overleaf-go/services/docstore/pkg/models"
	"github.com/das7pad/overleaf-go/services/docstore/pkg/types"
)

func newHttpController(dm docstore.Manager) httpController {
	return httpController{dm: dm}
}

type httpController struct {
	dm docstore.Manager
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

	projectRouter.
		NewRoute().
		Methods(http.MethodGet).
		Path("/doc-deleted").
		HandlerFunc(h.peakDeletedDocNames)
	projectRouter.
		NewRoute().
		Methods(http.MethodGet).
		Path("/doc").
		HandlerFunc(h.getAllDocContents)
	projectRouter.
		NewRoute().
		Methods(http.MethodGet).
		Path("/ranges").
		HandlerFunc(h.getAllRanges)
	projectRouter.
		NewRoute().
		Methods(http.MethodPost).
		Path("/archive").
		HandlerFunc(h.archiveProject)
	//goland:noinspection SpellCheckingInspection
	projectRouter.
		NewRoute().
		Methods(http.MethodPost).
		Path("/unarchive").
		HandlerFunc(h.unArchiveProject)
	projectRouter.
		NewRoute().
		Methods(http.MethodPost).
		Path("/destroy").
		HandlerFunc(h.destroyProject)

	docRouter.
		NewRoute().
		Methods(http.MethodGet).
		Path("").
		HandlerFunc(h.getDoc)
	docRouter.
		NewRoute().
		Methods(http.MethodGet).
		Path("/raw").
		HandlerFunc(h.getDocRaw)
	docRouter.
		NewRoute().
		Methods(http.MethodGet).
		Path("/deleted").
		HandlerFunc(h.isDocDeleted)
	docRouter.
		NewRoute().
		Methods(http.MethodPost).
		Path("").
		HandlerFunc(h.updateDoc)
	docRouter.
		NewRoute().
		Methods(http.MethodPatch).
		Path("").
		HandlerFunc(h.patchDoc)
	docRouter.
		NewRoute().
		Methods(http.MethodPost).
		Path("/archive").
		HandlerFunc(h.archiveDoc)

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

func getParam(r *http.Request, name string) string {
	return mux.Vars(r)[name]
}
func getRawIdFromRequest(r *http.Request, name string) string {
	return getParam(r, name)
}

func getId(r *http.Request, name string) primitive.ObjectID {
	id := r.Context().Value(name)
	if id == nil {
		// The validation middleware should have blocked this request.
		log.Printf(
			"%s not validated on route %s %s",
			name, r.Method, r.URL.Path,
		)
		panic("broken id validation")
	}
	return id.(primitive.ObjectID)
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
		if errors.IsBodyTooLargeError(err) {
			errorResponse(w, http.StatusRequestEntityTooLarge, err.Error())
			return
		}
		if errors.IsDocNotFoundError(err) {
			errorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		if errors.IsValidationError(err) {
			errorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		log.Printf("%s %s: %s: %v", r.Method, r.URL.Path, msg, err)
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
	_, _ = w.Write([]byte("docstore is alive (go)\n"))
}

func (h *httpController) peakDeletedDocNames(w http.ResponseWriter, r *http.Request) {
	limit := docstore.DefaultLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if limitRaw, err := strconv.ParseInt(raw, 10, 64); err == nil {
			limit = types.Limit(limitRaw)
		}
	}

	docNames, err := h.dm.PeakDeletedDocNames(
		r.Context(),
		getId(r, "projectId"),
		limit,
	)
	respond(w, r, http.StatusOK, docNames, err, "cannot peak deleted doc names")
}

func (h *httpController) getAllDocContents(w http.ResponseWriter, r *http.Request) {
	docs, err := h.dm.GetAllDocContents(
		r.Context(),
		getId(r, "projectId"),
	)
	respond(w, r, http.StatusOK, docs, err, "cannot get all doc contents")
}

func (h *httpController) getAllRanges(w http.ResponseWriter, r *http.Request) {
	docNames, err := h.dm.GetAllRanges(
		r.Context(),
		getId(r, "projectId"),
	)
	respond(w, r, http.StatusOK, docNames, err, "cannot get all ranges")
}

func (h *httpController) archiveProject(w http.ResponseWriter, r *http.Request) {
	err := h.dm.ArchiveProject(
		r.Context(),
		getId(r, "projectId"),
	)
	respond(w, r, http.StatusNoContent, nil, err, "cannot archive project")
}

func (h *httpController) unArchiveProject(w http.ResponseWriter, r *http.Request) {
	err := h.dm.UnArchiveProject(
		r.Context(),
		getId(r, "projectId"),
	)
	respond(w, r, http.StatusOK, nil, err, "cannot un-archive project")
}

func (h *httpController) destroyProject(w http.ResponseWriter, r *http.Request) {
	err := h.dm.DestroyProject(
		r.Context(),
		getId(r, "projectId"),
	)
	respond(w, r, http.StatusNoContent, nil, err, "cannot destroy project")
}

func (h *httpController) getDoc(w http.ResponseWriter, r *http.Request) {
	doc, err := h.dm.GetFullDoc(
		r.Context(),
		getId(r, "projectId"),
		getId(r, "docId"),
	)
	if err == nil {
		if doc.Deleted && r.URL.Query().Get("include_deleted") != "true" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
	}
	respond(w, r, http.StatusOK, doc, err, "cannot get full doc")
}

func (h *httpController) getDocRaw(w http.ResponseWriter, r *http.Request) {
	lines, err := h.dm.GetDocLines(
		r.Context(),
		getId(r, "projectId"),
		getId(r, "docId"),
	)
	if err != nil {
		respond(w, r, http.StatusOK, lines, err, "cannot get doc lines")
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(strings.Join(lines, "\n")))
}

type isDocDeletedResponseBody struct {
	Deleted bool `json:"deleted"`
}

func (h *httpController) isDocDeleted(w http.ResponseWriter, r *http.Request) {
	deleted, err := h.dm.IsDocDeleted(
		r.Context(),
		getId(r, "projectId"),
		getId(r, "docId"),
	)

	body := isDocDeletedResponseBody{}
	if err == nil {
		body.Deleted = deleted
	}
	respond(w, r, http.StatusOK, body, err, "cannot get delete state of doc")
}

type updateDocRequestBody struct {
	Lines   models.Lines    `json:"lines"`
	Ranges  models.Ranges   `json:"ranges"`
	Version *models.Version `json:"version"`
}
type updateDocResponseBody struct {
	Revision models.Revision   `json:"rev"`
	Modified docstore.Modified `json:"modified"`
}

func (h *httpController) updateDoc(w http.ResponseWriter, r *http.Request) {
	var requestBody updateDocRequestBody
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if requestBody.Version == nil {
		errorResponse(w, http.StatusBadRequest, "missing version")
		return
	}
	modified, revision, err := h.dm.UpdateDoc(
		r.Context(),
		getId(r, "projectId"),
		getId(r, "docId"),
		requestBody.Lines,
		*requestBody.Version,
		requestBody.Ranges,
	)

	body := updateDocResponseBody{}
	if err == nil {
		body.Revision = revision
		body.Modified = modified
	}
	respond(w, r, http.StatusOK, body, err, "cannot update doc")
}

func (h *httpController) patchDoc(w http.ResponseWriter, r *http.Request) {
	var requestBody models.DocMeta
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}
	err := h.dm.PatchDoc(
		r.Context(),
		getId(r, "projectId"),
		getId(r, "docId"),
		requestBody,
	)

	respond(w, r, http.StatusNoContent, nil, err, "cannot patch doc")
}

func (h *httpController) archiveDoc(w http.ResponseWriter, r *http.Request) {
	err := h.dm.ArchiveDoc(
		r.Context(),
		getId(r, "projectId"),
		getId(r, "docId"),
	)
	respond(w, r, http.StatusNoContent, nil, err, "cannot archive doc")
}
