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
	"github.com/gin-gonic/gin/binding"

	"github.com/das7pad/overleaf-go/pkg/httpUtils"
	"github.com/das7pad/overleaf-go/pkg/jwt/compileJWT"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
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

func (h *httpController) GetRouter(
	corsOptions httpUtils.CORSOptions,
) http.Handler {
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/status", h.status)
	router.HEAD("/status", h.status)

	internalRouter := router.Group("")
	internalProjectRouter := internalRouter.Group("/project/:projectId")
	internalProjectRouter.Use(httpUtils.ValidateAndSetId("projectId"))
	internalProjectUserRouter := internalProjectRouter.Group("/user/:userId")
	internalProjectUserRouter.Use(httpUtils.ValidateAndSetIdZeroOK("userId"))
	internalProjectUserRouter.GET("/editorLocals", h.editorLocals)

	internalUserRouter := internalRouter.Group("/user/:userId")
	internalUserRouter.Use(httpUtils.ValidateAndSetId("userId"))
	internalUserRouter.GET("/projectListLocals", h.projectListLocals)

	publicRouter := router.Group("")
	publicRouter.Use(httpUtils.CORS(corsOptions))
	publicRouter.Use(httpUtils.NoCache())

	publicApiRouter := publicRouter.Group("/api")
	publicApiRouter.POST("/login", h.login)
	publicApiRouter.POST("/logout", h.logout)

	jwtRouter := publicRouter.Group("/jwt/web")

	loggedInUserJWTRouter := jwtRouter.Group("")
	loggedInUserJWTRouter.Use(
		httpUtils.NewJWTHandler(h.wm.GetLoggedInUserJWTHandler()).Middleware(),
	)

	compileJWTRouter := jwtRouter.Group("/project/:projectId")
	compileJWTRouter.Use(
		httpUtils.NewJWTHandler(h.wm.GetCompileJWTHandler()).Middleware(),
	)
	compileJWTRouter.POST("/clear-cache", h.clearProjectCache)
	compileJWTRouter.POST("/compile", h.compileProject)
	compileJWTRouter.POST("/sync/code", h.syncFromCode)
	compileJWTRouter.POST("/sync/pdf", h.syncFromPDF)
	compileJWTRouter.POST("/wordcount", h.wordCount)

	compileJWTRouter.GET("/metadata", h.getMetadataForProject)

	compileJWTDocRouter := compileJWTRouter.Group("/doc/:docId")
	compileJWTDocRouter.Use(httpUtils.ValidateAndSetId("docId"))

	compileJWTDocRouter.POST("/metadata", h.getMetadataForDoc)

	loggedInUserJWTRouter.GET("/system/messages", h.getSystemMessages)
	return router
}

func mustGetSignedCompileProjectOptionsFromJwt(c *gin.Context) types.SignedCompileProjectRequestOptions {
	return compileJWT.MustGet(c).SignedCompileProjectRequestOptions
}

func (h *httpController) status(c *gin.Context) {
	c.String(http.StatusOK, "web is alive (go)\n")
}

type clearProjectCacheRequestBody struct {
	types.ClsiServerId `json:"clsiServerId"`
}

func (h *httpController) clearProjectCache(c *gin.Context) {
	request := &clearProjectCacheRequestBody{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	err := h.wm.ClearCache(
		c,
		mustGetSignedCompileProjectOptionsFromJwt(c),
		request.ClsiServerId,
	)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) compileProject(c *gin.Context) {
	request := &types.CompileProjectRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.SignedCompileProjectRequestOptions =
		mustGetSignedCompileProjectOptionsFromJwt(c)
	response := &types.CompileProjectResponse{}
	err := h.wm.Compile(
		c,
		request,
		response,
	)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) syncFromCode(c *gin.Context) {
	request := &types.SyncFromCodeRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.SignedCompileProjectRequestOptions =
		mustGetSignedCompileProjectOptionsFromJwt(c)

	response := &clsiTypes.PDFPositions{}
	err := h.wm.SyncFromCode(
		c,
		request,
		response,
	)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) syncFromPDF(c *gin.Context) {
	request := &types.SyncFromPDFRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.SignedCompileProjectRequestOptions =
		mustGetSignedCompileProjectOptionsFromJwt(c)

	response := &clsiTypes.CodePositions{}
	err := h.wm.SyncFromPDF(
		c,
		request,
		response,
	)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) wordCount(c *gin.Context) {
	request := &types.WordCountRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.SignedCompileProjectRequestOptions =
		mustGetSignedCompileProjectOptionsFromJwt(c)

	response := &clsiTypes.Words{}
	err := h.wm.WordCount(
		c,
		request,
		response,
	)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) editorLocals(c *gin.Context) {
	request := &types.LoadEditorRequest{
		ProjectId: httpUtils.GetId(c, "projectId"),
		UserId:    httpUtils.GetId(c, "userId"),
	}
	if err := c.MustBindWith(request, binding.Query); err != nil {
		return
	}
	response := &types.LoadEditorResponse{}
	t := &sharedTypes.Timed{}
	t.Begin()
	err := h.wm.LoadEditor(c, request, response)
	t.End()
	c.Header("Server-Timing", "editorLocals;dur="+t.MS())
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) projectListLocals(c *gin.Context) {
	request := &types.ProjectListRequest{
		UserId: httpUtils.GetId(c, "userId"),
	}
	response := &types.ProjectListResponse{}
	t := &sharedTypes.Timed{}
	t.Begin()
	err := h.wm.ProjectList(c, request, response)
	t.End()
	c.Header("Server-Timing", "projectListLocals;dur="+t.MS())
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) getSystemMessages(c *gin.Context) {
	m := h.wm.GetAllCached(c, httpUtils.GetId(c, "userId"))
	httpUtils.Respond(c, http.StatusOK, m, nil)
}

func (h *httpController) getMetadataForProject(c *gin.Context) {
	projectId := mustGetSignedCompileProjectOptionsFromJwt(c).ProjectId
	resp, err := h.wm.GetMetadataForProject(c, projectId)
	httpUtils.Respond(c, http.StatusOK, resp, err)
}

func (h *httpController) getMetadataForDoc(c *gin.Context) {
	projectId := mustGetSignedCompileProjectOptionsFromJwt(c).ProjectId
	docId := httpUtils.GetId(c, "docId")
	request := &types.ProjectDocMetadataRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	resp, err := h.wm.GetMetadataForDoc(c, projectId, docId, request)
	httpUtils.Respond(c, http.StatusOK, resp, err)
}

func (h *httpController) login(c *gin.Context) {
	resp := &types.LoginResponse{}
	s, err := h.wm.GetOrCreateSession(c)
	if err != nil {
		httpUtils.Respond(c, http.StatusOK, resp, err)
		return
	}
	request := &types.LoginRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.Session = s
	request.IPAddress = c.ClientIP()
	err = h.wm.Login(c, request, resp)
	if err2 := h.wm.Flush(c, s); err == nil && err2 != nil {
		err = err2
	}
	httpUtils.Respond(c, http.StatusOK, resp, err)
}

func (h *httpController) logout(c *gin.Context) {
	resp := &types.LogoutResponse{
		RedirectTo: "/login",
	}
	s, err := h.wm.GetOrCreateSession(c)
	if err != nil {
		httpUtils.Respond(c, http.StatusOK, resp, err)
		return
	}
	request := &types.LogoutRequest{Session: s}
	err = h.wm.LogOut(c, request)
	if err2 := h.wm.Flush(c, s); err == nil && err2 != nil {
		err = err2
	}
	httpUtils.Respond(c, http.StatusOK, resp, err)
}
