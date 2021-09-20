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
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"

	"github.com/das7pad/overleaf-go/pkg/httpUtils"
	clsiTypes "github.com/das7pad/overleaf-go/services/clsi/pkg/types"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func newHttpController(wm web.Manager) httpController {
	return httpController{wm: wm}
}

type httpController struct {
	wm web.Manager
}

const (
	userIdField    = "userId"
	projectIdField = "projectId"
)

func (h *httpController) GetRouter(
	corsOptions httpUtils.CORSOptions,
	jwtOptions httpUtils.JWTOptions,
	client redis.UniversalClient,
) http.Handler {
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/status", h.status)
	router.HEAD("/status", h.status)

	jwtRouter := router.Group("/jwt/web")
	jwtRouter.Use(httpUtils.CORS(corsOptions))
	jwtRouter.Use(httpUtils.NoCache())
	jwtRouter.Use(httpUtils.NewJWTHandler(jwtOptions).Middleware())
	jwtRouter.Use(httpUtils.ValidateAndSetJWTId(userIdField))

	projectRouter := jwtRouter.Group("/project/:projectId")
	projectRouter.Use(httpUtils.ValidateAndSetJWTId(projectIdField))
	projectRouter.Use(httpUtils.CheckEpochs(client))
	projectRouter.POST("/clear-cache", h.clearProjectCache)
	projectRouter.POST("/compile", h.compileProject)
	projectRouter.POST("/sync/code", h.syncFromCode)
	projectRouter.POST("/sync/pdf", h.syncFromPDF)
	projectRouter.POST("/wordcount", h.wordCount)
	return router
}

func mustGetSignedCompileProjectOptionsFromJwt(c *gin.Context) *types.SignedCompileProjectRequestOptions {
	compileGroupRaw, err := httpUtils.GetStringFromJwt(c, "compileGroup")
	if err != nil {
		httpUtils.RespondErr(c, err)
		return nil
	}
	timeoutRaw, err := httpUtils.GetDurationFromJwt(c, "timeout")
	if err != nil {
		httpUtils.RespondErr(c, err)
		return nil
	}
	return &types.SignedCompileProjectRequestOptions{
		ProjectId:    httpUtils.GetId(c, "projectId"),
		UserId:       httpUtils.GetId(c, "userId"),
		CompileGroup: clsiTypes.CompileGroup(compileGroupRaw),
		Timeout:      clsiTypes.Timeout(timeoutRaw),
	}
}

func (h *httpController) status(c *gin.Context) {
	c.String(http.StatusOK, "web is alive (go)\n")
}

type clearProjectCacheRequestBody struct {
	types.ClsiServerId `json:"clsiServerId"`
}

func (h *httpController) clearProjectCache(c *gin.Context) {
	so := mustGetSignedCompileProjectOptionsFromJwt(c)
	if so == nil {
		return
	}

	request := &clearProjectCacheRequestBody{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	err := h.wm.ClearProjectCache(
		c,
		*so,
		request.ClsiServerId,
	)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) compileProject(c *gin.Context) {
	so := mustGetSignedCompileProjectOptionsFromJwt(c)
	if so == nil {
		return
	}

	request := &types.CompileProjectRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.SignedCompileProjectRequestOptions = *so
	response := &types.CompileProjectResponse{}
	err := h.wm.CompileProject(
		c,
		request,
		response,
	)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) syncFromCode(c *gin.Context) {
	so := mustGetSignedCompileProjectOptionsFromJwt(c)
	if so == nil {
		return
	}

	request := &types.SyncFromCodeRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.SignedCompileProjectRequestOptions = *so

	response := &clsiTypes.PDFPositions{}
	err := h.wm.SyncFromCode(
		c,
		request,
		response,
	)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) syncFromPDF(c *gin.Context) {
	so := mustGetSignedCompileProjectOptionsFromJwt(c)
	if so == nil {
		return
	}

	request := &types.SyncFromPDFRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.SignedCompileProjectRequestOptions = *so

	response := &clsiTypes.CodePositions{}
	err := h.wm.SyncFromPDF(
		c,
		request,
		response,
	)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) wordCount(c *gin.Context) {
	so := mustGetSignedCompileProjectOptionsFromJwt(c)
	if so == nil {
		return
	}

	request := &types.WordCountRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.SignedCompileProjectRequestOptions = *so

	response := &clsiTypes.Words{}
	err := h.wm.WordCount(
		c,
		request,
		response,
	)
	httpUtils.Respond(c, http.StatusOK, response, err)
}
