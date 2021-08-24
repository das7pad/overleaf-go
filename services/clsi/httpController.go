// Golang port of the Overleaf clsi service
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

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/constants"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

func newHttpController(cm clsi.Manager) httpController {
	return httpController{cm: cm}
}

type httpController struct {
	cm clsi.Manager
}

func (h *httpController) GetRouter() http.Handler {
	router := mux.NewRouter()
	router.HandleFunc("/status", h.status)
	router.HandleFunc("/health_check", h.healthCheck)

	projectRouter := router.
		PathPrefix("/project/{projectId}").
		Subrouter()
	projectRouter.Use(validateAndSetId("projectId"))

	userRouter := projectRouter.
		PathPrefix("/user/{userId}").
		Subrouter()
	userRouter.Use(validateAndSetId("userId"))

	anonymousRouter := projectRouter.
		PathPrefix("").
		Subrouter()
	anonymousRouter.Use(setAnonymousUserId())

	compileRouters := []*mux.Router{
		anonymousRouter,
		userRouter,
	}
	for _, compileRouter := range compileRouters {
		compileRouter.
			NewRoute().
			Methods(http.MethodPost).
			Path("/compile").
			HandlerFunc(h.compile)
		compileRouter.
			NewRoute().
			Methods(http.MethodPost).
			Path("/compile/stop").
			HandlerFunc(h.stopCompile)
		compileRouter.
			NewRoute().
			Methods(http.MethodDelete).
			Path("").
			HandlerFunc(h.clearCache)
		compileRouter.
			NewRoute().
			Methods(http.MethodGet).
			Path("/sync/code").
			HandlerFunc(h.syncFromCode)
		compileRouter.
			NewRoute().
			Methods(http.MethodGet).
			Path("/sync/pdf").
			HandlerFunc(h.syncFromPDF)
		//goland:noinspection SpellCheckingInspection
		compileRouter.
			NewRoute().
			Methods(http.MethodGet).
			Path("/wordcount").
			HandlerFunc(h.wordCount)
		compileRouter.
			NewRoute().
			Methods(http.MethodGet, http.MethodPost).
			Path("/status").
			HandlerFunc(h.cookieStatus)
	}

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
func setAnonymousUserId() mux.MiddlewareFunc {
	id := primitive.NilObjectID
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), "userId", id)
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

func decodeFromQuery(w http.ResponseWriter, r *http.Request, name string, targetType interface{}, target interface{}) bool {
	raw := r.URL.Query().Get(name)
	if _, isStringTarget := targetType.(types.StringParameter); isStringTarget {
		raw = "\"" + raw + "\""
	}
	if err := json.Unmarshal([]byte(raw), &target); err != nil {
		errorResponse(
			w,
			http.StatusBadRequest,
			"bad parameter: "+name,
		)
		return false
	}
	return true
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
		if errors.IsMissingOutputFileError(err) {
			errorResponse(w, http.StatusNotFound, err.Error())
			return
		}
		if errors.IsInvalidState(err) {
			errorResponse(w, http.StatusConflict, err.Error())
			return
		}
		if errors.IsAlreadyCompiling(err) {
			errorResponse(w, http.StatusLocked, err.Error())
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
	_, _ = w.Write([]byte("clsi is alive (go)\n"))
}

type compileRequestBody struct {
	Request *types.CompileRequest `json:"compile"`
}
type compileResponseBody struct {
	Response *types.CompileResponse `json:"compile"`
}

func (h *httpController) compile(w http.ResponseWriter, r *http.Request) {
	var requestBody compileRequestBody
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if requestBody.Request == nil {
		errorResponse(w, http.StatusBadRequest, "missing compile request")
		return
	}

	response := types.CompileResponse{
		Status:      constants.Failure,
		OutputFiles: make(types.OutputFiles, 0),
	}
	err := h.cm.Compile(
		r.Context(),
		getId(r, "projectId"),
		getId(r, "userId"),
		requestBody.Request,
		&response,
	)
	if err != nil {
		response.Error = "hidden"
	}
	body := &compileResponseBody{Response: &response}
	respond(w, r, http.StatusOK, body, err, "cannot compile")
}

func (h *httpController) stopCompile(w http.ResponseWriter, r *http.Request) {
	err := h.cm.StopCompile(
		r.Context(),
		getId(r, "projectId"),
		getId(r, "userId"),
	)
	respond(w, r, http.StatusNoContent, nil, err, "cannot stop compile")
}

func (h *httpController) clearCache(w http.ResponseWriter, r *http.Request) {
	err := h.cm.ClearCache(
		r.Context(),
		getId(r, "projectId"),
		getId(r, "userId"),
	)
	respond(w, r, http.StatusNoContent, nil, err, "cannot clear cache")
}

func (h *httpController) parseSyncTexOptions(w http.ResponseWriter, r *http.Request) *types.SyncTexOptions {
	o := types.SyncTexOptions{}
	if !decodeFromQuery(w, r, "imageName", o.ImageName, &o.ImageName) {
		return nil
	}
	if !decodeFromQuery(w, r, "buildId", o.BuildId, &o.BuildId) {
		return nil
	}
	if !decodeFromQuery(w, r, "compileGroup", o.CompileGroup, &o.CompileGroup) {
		return nil
	}
	return &o
}

type syncFromCodeResponseBody struct {
	PDFPositions *types.PDFPositions `json:"pdf"`
}

func (h *httpController) syncFromCode(w http.ResponseWriter, r *http.Request) {
	// TODO: refactor into POST request
	var file types.FileName
	if !decodeFromQuery(w, r, "file", file, &file) {
		return
	}
	var row types.Row
	if !decodeFromQuery(w, r, "line", row, &row) {
		return
	}
	var column types.Column
	if !decodeFromQuery(w, r, "column", column, &column) {
		return
	}
	syncTexOptions := h.parseSyncTexOptions(w, r)
	if syncTexOptions == nil {
		return
	}
	request := types.SyncFromCodeRequest{
		SyncTexOptions: syncTexOptions,
		FileName:       file,
		Row:            row,
		Column:         column,
	}

	pdfPositions := types.PDFPositions{}
	err := h.cm.SyncFromCode(
		r.Context(),
		getId(r, "projectId"),
		getId(r, "userId"),
		&request,
		&pdfPositions,
	)
	body := &syncFromCodeResponseBody{}
	if err == nil {
		body.PDFPositions = &pdfPositions
	}
	respond(w, r, http.StatusOK, body, err, "cannot sync from code")
}

type syncFromPDFResponseBody struct {
	CodePositions *types.CodePositions `json:"code"`
}

func (h *httpController) syncFromPDF(w http.ResponseWriter, r *http.Request) {
	// TODO: refactor into POST request
	var page types.Page
	if !decodeFromQuery(w, r, "page", page, &page) {
		return
	}
	var horizontal types.Horizontal
	if !decodeFromQuery(w, r, "h", horizontal, &horizontal) {
		return
	}
	var vertical types.Vertical
	if !decodeFromQuery(w, r, "v", vertical, &vertical) {
		return
	}
	syncTexOptions := h.parseSyncTexOptions(w, r)
	if syncTexOptions == nil {
		return
	}
	request := types.SyncFromPDFRequest{
		SyncTexOptions: syncTexOptions,
		Page:           page,
		Horizontal:     horizontal,
		Vertical:       vertical,
	}

	codePositions := types.CodePositions{}
	err := h.cm.SyncFromPDF(
		r.Context(),
		getId(r, "projectId"),
		getId(r, "userId"),
		&request,
		&codePositions,
	)
	body := &syncFromPDFResponseBody{}
	if err == nil {
		body.CodePositions = &codePositions
	}
	respond(w, r, http.StatusOK, body, err, "cannot sync from pdf")
}

type wordCountResponseBody struct {
	NWords *types.Words `json:"texcount"`
}

func (h *httpController) wordCount(w http.ResponseWriter, r *http.Request) {
	// TODO: refactor into POST request
	var request types.WordCountRequest
	if !decodeFromQuery(w, r, "file", request.FileName, &request.FileName) {
		return
	}
	if !decodeFromQuery(w, r, "image", request.ImageName, &request.ImageName) {
		return
	}

	var words types.Words
	err := h.cm.WordCount(
		r.Context(),
		getId(r, "projectId"),
		getId(r, "userId"),
		&request,
		&words,
	)
	body := &wordCountResponseBody{}
	if err == nil {
		body.NWords = &words
	}
	respond(w, r, http.StatusOK, body, err, "cannot count words")
}

func (h *httpController) cookieStatus(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (h *httpController) healthCheck(w http.ResponseWriter, r *http.Request) {
	err := h.cm.HealthCheck(r.Context())
	msg := "health check failed"
	respond(w, r, http.StatusOK, nil, err, msg)
}
