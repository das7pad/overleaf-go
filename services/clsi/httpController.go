// Golang port of Overleaf
// Copyright (C) 2021-2022 Jakob Ackermann <das7pad@outlook.com>
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
	"net/http"

	"github.com/das7pad/overleaf-go/pkg/httpUtils"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/constants"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

func newHTTPController(cm clsi.Manager) httpController {
	return httpController{cm: cm}
}

type httpController struct {
	cm clsi.Manager
}

func (h *httpController) GetRouter() *httpUtils.Router {
	router := httpUtils.NewRouter(&httpUtils.RouterOptions{})
	router.GET("/health_check", h.healthCheck)
	router.HEAD("/health_check", h.healthCheck)

	projectRouter := router.Group("/project/{projectId}")
	projectRouter.Use(httpUtils.ValidateAndSetId("projectId"))
	userRouter := projectRouter.Group("/user/{userId}")
	userRouter.Use(httpUtils.ValidateAndSetIdZeroOK("userId"))

	userRouter.POST("/compile", h.compile)
	userRouter.POST("/compile/stop", h.stopCompile)
	userRouter.DELETE("", h.clearCache)
	userRouter.POST("/sync/code", h.syncFromCode)
	userRouter.POST("/sync/pdf", h.syncFromPDF)
	userRouter.POST("/wordcount", h.wordCount)
	userRouter.GET("/status", h.cookieStatus)
	userRouter.POST("/status", h.cookieStatus)

	return router
}

type compileRequestBody struct {
	Request types.CompileRequest `json:"compile" binding:"required"`
}
type compileResponseBody struct {
	Response *types.CompileResponse `json:"compile"`
}

func (h *httpController) compile(c *httpUtils.Context) {
	requestBody := &compileRequestBody{}
	if !httpUtils.MustParseJSON(requestBody, c) {
		return
	}

	response := types.CompileResponse{
		Status:      constants.Failure,
		OutputFiles: make(types.OutputFiles, 0),
	}
	err := h.cm.Compile(
		c,
		httpUtils.GetId(c, "projectId"),
		httpUtils.GetId(c, "userId"),
		&requestBody.Request,
		&response,
	)
	if err != nil {
		response.Error = "hidden"
	}
	body := &compileResponseBody{Response: &response}
	httpUtils.Respond(c, http.StatusOK, body, err)
}

func (h *httpController) stopCompile(c *httpUtils.Context) {
	err := h.cm.StopCompile(
		c,
		httpUtils.GetId(c, "projectId"),
		httpUtils.GetId(c, "userId"),
	)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) clearCache(c *httpUtils.Context) {
	err := h.cm.ClearCache(
		httpUtils.GetId(c, "projectId"),
		httpUtils.GetId(c, "userId"),
	)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) syncFromCode(c *httpUtils.Context) {
	request := &types.SyncFromCodeRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}

	body := make(types.PDFPositions, 0)
	err := h.cm.SyncFromCode(
		c,
		httpUtils.GetId(c, "projectId"),
		httpUtils.GetId(c, "userId"),
		request,
		&body,
	)
	httpUtils.Respond(c, http.StatusOK, body, err)
}

func (h *httpController) syncFromPDF(c *httpUtils.Context) {
	request := &types.SyncFromPDFRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}

	body := make(types.CodePositions, 0)
	err := h.cm.SyncFromPDF(
		c,
		httpUtils.GetId(c, "projectId"),
		httpUtils.GetId(c, "userId"),
		request,
		&body,
	)
	httpUtils.Respond(c, http.StatusOK, body, err)
}

func (h *httpController) wordCount(c *httpUtils.Context) {
	request := &types.WordCountRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}

	var body types.Words
	err := h.cm.WordCount(
		c,
		httpUtils.GetId(c, "projectId"),
		httpUtils.GetId(c, "userId"),
		request,
		&body,
	)
	httpUtils.Respond(c, http.StatusOK, body, err)
}

func (h *httpController) cookieStatus(c *httpUtils.Context) {
	request := &types.StartInBackgroundRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	err := h.cm.StartInBackground(
		c,
		httpUtils.GetId(c, "projectId"),
		httpUtils.GetId(c, "userId"),
		request,
	)
	httpUtils.Respond(c, http.StatusOK, nil, err)
}

func (h *httpController) healthCheck(c *httpUtils.Context) {
	err := h.cm.HealthCheck(c)
	httpUtils.Respond(c, http.StatusOK, nil, err)
}
