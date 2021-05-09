// Golang port of the Overleaf filestore service
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
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"

	"github.com/das7pad/filestore/pkg/backend"

	"github.com/das7pad/filestore/pkg/managers/filestore"
	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func newHttpController(fm filestore.Manager, allowRedirects bool) httpController {
	return httpController{fm: fm, allowRedirects: allowRedirects}
}

type httpController struct {
	fm             filestore.Manager
	allowRedirects bool
}

func (h *httpController) GetRouter() http.Handler {
	router := mux.NewRouter()
	router.HandleFunc("/status", h.status)

	projectRouter := router.
		PathPrefix("/project/{projectId}").
		Subrouter()
	projectRouter.Use(validateAndSetId("projectId"))
	projectFileRouter := projectRouter.
		PathPrefix("/file/{fileId}").
		Subrouter()
	projectFileRouter.Use(validateAndSetId("fileId"))

	projectRouter.
		NewRoute().
		Methods(http.MethodDelete).
		Path("").
		HandlerFunc(h.deleteProject)
	projectRouter.
		NewRoute().
		Methods(http.MethodGet).
		Path("/size").
		HandlerFunc(h.getProjectSize)

	projectFileRouter.
		NewRoute().
		Methods(http.MethodDelete).
		Path("").
		HandlerFunc(h.deleteProjectFile)
	projectFileRouter.
		NewRoute().
		Methods(http.MethodGet).
		Path("").
		HandlerFunc(h.getProjectFile)
	projectFileRouter.
		NewRoute().
		Methods(http.MethodHead).
		Path("").
		HandlerFunc(h.getProjectFileHEAD)
	projectFileRouter.
		NewRoute().
		Methods(http.MethodPost).
		Path("").
		HandlerFunc(h.sendProjectFile)
	projectFileRouter.
		NewRoute().
		Methods(http.MethodPut).
		Path("").
		HandlerFunc(h.copyProjectFile)
	projectFileRouter.
		NewRoute().
		Methods(http.MethodPut).
		Path("/upload").
		HandlerFunc(h.sendProjectFileViaPUT)
	projectFileRouter.
		NewRoute().
		Methods(http.MethodPut).
		Path("/copy").
		HandlerFunc(h.copyProjectFile)
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
	w.Header().Set("X-Coded", "true")
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
		if _, is400 := err.(filestore.ValidationError); is400 {
			errorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		if _, is404 := err.(backend.ErrorNotFound); is404 {
			errorResponse(w, http.StatusNotFound, err.Error())
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

func redirect(
	w http.ResponseWriter,
	r *http.Request,
	u *url.URL,
	err error,
	msg string,
) {
	if err == nil {
		w.Header().Set("Location", u.String())
	}
	respond(w, r, http.StatusTemporaryRedirect, nil, err, msg)
}

func (h *httpController) status(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("filestore is alive (go)\n"))
}

func (h *httpController) deleteProject(w http.ResponseWriter, r *http.Request) {
	err := h.fm.DeleteProject(
		r.Context(),
		getId(r, "projectId"),
	)
	respond(w, r, http.StatusNoContent, nil, err, "cannot delete project")
}

type getProjectSizeResponseBody struct {
	TotalSize int64 `json:"total bytes"`
}

func (h *httpController) getProjectSize(w http.ResponseWriter, r *http.Request) {
	s, err := h.fm.GetSizeOfProject(
		r.Context(),
		getId(r, "projectId"),
	)
	body := getProjectSizeResponseBody{TotalSize: s}
	respond(w, r, http.StatusOK, body, err, "cannot get project size")
}

func (h *httpController) deleteProjectFile(w http.ResponseWriter, r *http.Request) {
	err := h.fm.DeleteProjectFile(
		r.Context(),
		getId(r, "projectId"),
		getId(r, "fileId"),
	)
	respond(w, r, http.StatusNoContent, nil, err, "cannot delete project file")
}

func (h *httpController) getProjectFile(w http.ResponseWriter, r *http.Request) {
	if h.allowRedirects {
		u, err := h.fm.GetRedirectURLForGETOnProjectFile(
			r.Context(),
			getId(r, "projectId"),
			getId(r, "fileId"),
		)
		redirect(w, r, u, err, "cannot redirect GET on project file")
		return
	}
	options := backend.GetOptions{}
	body, err := h.fm.GetReadStreamForProjectFile(
		r.Context(),
		getId(r, "projectId"),
		getId(r, "fileId"),
		options,
	)
	if err != nil {
		respond(w, r, http.StatusOK, nil, err, "cannot GET project file")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	_, _ = io.Copy(w, body)
}

func (h *httpController) getProjectFileHEAD(w http.ResponseWriter, r *http.Request) {
	if h.allowRedirects {
		u, err := h.fm.GetRedirectURLForHEADOnProjectFile(
			r.Context(),
			getId(r, "projectId"),
			getId(r, "fileId"),
		)
		redirect(w, r, u, err, "cannot redirect HEAD on project file")
		return
	}
	size, err := h.fm.GetSizeOfProjectFile(
		r.Context(),
		getId(r, "projectId"),
		getId(r, "fileId"),
	)
	if err == nil {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", size))
	}
	respond(w, r, http.StatusOK, nil, err, "cannot get project file size")
}

func getBodySize(r *http.Request) int64 {
	raw := r.Header.Get("Content-Length")
	if raw == "" {
		return -1
	}
	bodySize, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return -1
	}
	return bodySize
}

func (h *httpController) sendProjectFile(w http.ResponseWriter, r *http.Request) {
	// NOTE: This is a POST request. We cannot redirect to a PUT URL.
	//       Redirecting to a singed POST URL does not work unless additional
	//        POST form data is amended.
	//       This needs an API rework for uploading via PUT.

	options := backend.SendOptions{
		ContentSize:     getBodySize(r),
		ContentEncoding: r.Header.Get("Content-Encoding"),
		ContentType:     r.Header.Get("Content-Type"),
	}
	err := h.fm.SendStreamForProjectFile(
		r.Context(),
		getId(r, "projectId"),
		getId(r, "fileId"),
		r.Body,
		options,
	)
	respond(w, r, http.StatusOK, nil, err, "cannot POST project file")
}

func (h *httpController) sendProjectFileViaPUT(w http.ResponseWriter, r *http.Request) {
	if h.allowRedirects {
		u, err := h.fm.GetRedirectURLForPUTOnProjectFile(
			r.Context(),
			getId(r, "projectId"),
			getId(r, "fileId"),
		)
		redirect(w, r, u, err, "cannot redirect PUT on project file")
		return
	}

	options := backend.SendOptions{
		ContentSize:     getBodySize(r),
		ContentEncoding: r.Header.Get("Content-Encoding"),
		ContentType:     r.Header.Get("Content-Type"),
	}
	err := h.fm.SendStreamForProjectFile(
		r.Context(),
		getId(r, "projectId"),
		getId(r, "fileId"),
		r.Body,
		options,
	)
	respond(w, r, http.StatusOK, nil, err, "cannot PUT project file")
}

type copyProjectRequestBody struct {
	Source struct {
		ProjectId primitive.ObjectID `json:"project_id"`
		FileId    primitive.ObjectID `json:"file_id"`
	} `json:"source"`
}

func (h *httpController) copyProjectFile(w http.ResponseWriter, r *http.Request) {
	var requestBody copyProjectRequestBody
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if requestBody.Source.ProjectId == primitive.NilObjectID {
		errorResponse(w, http.StatusBadRequest, "source.project_id missing")
		return
	}
	if requestBody.Source.FileId == primitive.NilObjectID {
		errorResponse(w, http.StatusBadRequest, "source.file_id missing")
		return
	}
	err := h.fm.CopyProjectFile(
		r.Context(),
		requestBody.Source.ProjectId,
		requestBody.Source.FileId,
		getId(r, "projectId"),
		getId(r, "fileId"),
	)
	respond(w, r, http.StatusOK, nil, err, "cannot copy project file")
}
