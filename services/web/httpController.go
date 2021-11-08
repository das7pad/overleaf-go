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
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/httpUtils"
	"github.com/das7pad/overleaf-go/pkg/jwt/projectJWT"
	"github.com/das7pad/overleaf-go/pkg/models/project"
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
	publicApiRouter.POST("/beta/opt-in", h.optInBetaProgram)
	publicApiRouter.POST("/beta/opt-out", h.optOutBetaProgram)
	publicApiRouter.POST("/grant/ro/:token", h.grantTokenAccessReadOnly)
	publicApiRouter.POST("/grant/rw/:token", h.grantTokenAccessReadAndWrite)
	publicApiRouter.GET("/user/contacts", h.getUserContacts)
	publicApiRouter.POST("/user/jwt", h.getLoggedInUserJWT)
	publicApiRouter.GET("/user/projects", h.getUserProjects)
	publicApiRouter.POST("/login", h.login)
	publicApiRouter.POST("/logout", h.logout)

	{
		// Tag routes
		r := publicApiRouter.Group("/tag")
		r.POST("", h.createTag)

		rt := r.Group("/:tagId")
		rt.Use(httpUtils.ValidateAndSetId("tagId"))
		rt.DELETE("", h.deleteTag)
		rt.POST("/rename", h.renameTag)

		rtp := rt.Group("/project/:projectId")
		rtp.Use(httpUtils.ValidateAndSetId("projectId"))
		rtp.DELETE("", h.removeProjectToTag)
		rtp.POST("", h.addProjectToTag)
	}

	{
		// Project routes with session auth
		r := publicApiRouter.Group("/project/:projectId")
		r.Use(httpUtils.ValidateAndSetId("projectId"))
		r.DELETE("/archive", h.unArchiveProject)
		r.POST("/archive", h.archiveProject)
		r.GET("/entities", h.getProjectEntities)
		r.POST("/jwt", h.getProjectJWT)
		r.DELETE("/trash", h.unTrashProject)
		r.POST("/trash", h.trashProject)
		r.POST("/ws/bootstrap", h.getWSBootstrap)

		rFile := r.Group("/file/:fileId")
		rFile.Use(httpUtils.ValidateAndSetId("fileId"))
		rFile.GET("", h.getProjectFile)
		rFile.HEAD("", h.getProjectFileSize)
	}

	jwtRouter := publicRouter.Group("/jwt/web")

	loggedInUserJWTRouter := jwtRouter.Group("")
	loggedInUserJWTRouter.Use(
		httpUtils.NewJWTHandler(h.wm.GetLoggedInUserJWTHandler()).Middleware(),
	)

	projectJWTRouter := jwtRouter.Group("/project/:projectId")
	projectJWTRouter.Use(
		httpUtils.NewJWTHandler(h.wm.GetProjectJWTHandler()).Middleware(),
	)
	projectJWTRouter.POST("/clear-cache", h.clearProjectCache)
	projectJWTRouter.POST("/compile", h.compileProject)
	projectJWTRouter.POST("/sync/code", h.syncFromCode)
	projectJWTRouter.POST("/sync/pdf", h.syncFromPDF)
	projectJWTRouter.POST("/wordcount", h.wordCount)

	projectJWTRouter.GET("/metadata", h.getMetadataForProject)

	{
		// block access for token users with readOnly project access
		r := projectJWTRouter.Group("")
		r.Use(blockRestrictedUsers)
		r.GET("/messages", h.getProjectMessages)
		r.POST("/messages", h.sendProjectMessage)
	}

	projectJWTDocRouter := projectJWTRouter.Group("/doc/:docId")
	projectJWTDocRouter.Use(httpUtils.ValidateAndSetId("docId"))

	projectJWTDocRouter.POST("/metadata", h.getMetadataForDoc)

	loggedInUserJWTRouter.GET("/system/messages", h.getSystemMessages)
	return router
}

var (
	err403 = &errors.NotAuthorizedError{}
)

func blockRestrictedUsers(c *gin.Context) {
	if projectJWT.MustGet(c).IsRestrictedUser() {
		httpUtils.Respond(c, http.StatusOK, nil, err403)
		return
	}
}

func mustGetSignedCompileProjectOptionsFromJwt(c *gin.Context) types.SignedCompileProjectRequestOptions {
	return projectJWT.MustGet(c).SignedCompileProjectRequestOptions
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

func (h *httpController) getUserProjects(c *gin.Context) {
	resp := &types.GetUserProjectsResponse{}
	s, err := h.wm.GetOrCreateSession(c)
	if err != nil {
		httpUtils.Respond(c, http.StatusOK, resp, err)
		return
	}
	request := &types.GetUserProjectsRequest{Session: s}
	err = h.wm.GetUserProjects(c, request, resp)
	httpUtils.Respond(c, http.StatusOK, resp, err)
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

func (h *httpController) getLoggedInUserJWT(c *gin.Context) {
	resp := types.GetLoggedInUserJWTResponse("")
	s, err := h.wm.GetOrCreateSession(c)
	if err != nil {
		httpUtils.Respond(c, http.StatusOK, resp, err)
		return
	}
	request := &types.GetLoggedInUserJWTRequest{
		Session: s,
	}
	err = h.wm.GetLoggedInUserJWT(c, request, &resp)
	httpUtils.Respond(c, http.StatusOK, resp, err)
}

func (h *httpController) getProjectJWT(c *gin.Context) {
	resp := types.GetProjectJWTResponse("")
	s, err := h.wm.GetOrCreateSession(c)
	if err != nil {
		httpUtils.Respond(c, http.StatusOK, resp, err)
		return
	}
	request := &types.GetProjectJWTRequest{
		ProjectId: httpUtils.GetId(c, "projectId"),
		Session:   s,
	}
	err = h.wm.GetProjectJWT(c, request, &resp)
	httpUtils.Respond(c, http.StatusOK, resp, err)
}

func (h *httpController) getWSBootstrap(c *gin.Context) {
	resp := types.GetWSBootstrapResponse{}
	s, err := h.wm.GetOrCreateSession(c)
	if err != nil {
		httpUtils.Respond(c, http.StatusOK, resp, err)
		return
	}
	request := &types.GetWSBootstrapRequest{
		ProjectId: httpUtils.GetId(c, "projectId"),
		Session:   s,
	}
	err = h.wm.GetWSBootstrap(c, request, &resp)
	httpUtils.Respond(c, http.StatusOK, resp, err)
}

func (h *httpController) getProjectMessages(c *gin.Context) {
	request := &types.GetProjectChatMessagesRequest{}
	if err := c.MustBindWith(request, binding.Query); err != nil {
		return
	}
	request.ProjectId = projectJWT.MustGet(c).ProjectId
	response := &types.GetProjectChatMessagesResponse{}
	err := h.wm.GetProjectMessages(c, request, response)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) sendProjectMessage(c *gin.Context) {
	request := &types.SendProjectChatMessageRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.ProjectId = projectJWT.MustGet(c).ProjectId
	request.UserId = projectJWT.MustGet(c).UserId
	err := h.wm.SendProjectMessage(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) optInBetaProgram(c *gin.Context) {
	s, err := h.wm.GetOrCreateSession(c)
	if err != nil {
		httpUtils.Respond(c, http.StatusOK, nil, err)
		return
	}
	request := &types.OptInBetaProgramRequest{Session: s}
	err = h.wm.OptInBetaProgram(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) optOutBetaProgram(c *gin.Context) {
	s, err := h.wm.GetOrCreateSession(c)
	if err != nil {
		httpUtils.Respond(c, http.StatusOK, nil, err)
		return
	}
	request := &types.OptOutBetaProgramRequest{Session: s}
	err = h.wm.OptOutBetaProgram(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) getProjectEntities(c *gin.Context) {
	resp := &types.GetProjectEntitiesResponse{}
	s, err := h.wm.GetOrCreateSession(c)
	if err != nil {
		httpUtils.Respond(c, http.StatusOK, resp, err)
		return
	}
	request := &types.GetProjectEntitiesRequest{
		Session:   s,
		ProjectId: httpUtils.GetId(c, "projectId"),
	}
	err = h.wm.GetProjectEntities(c, request, resp)
	httpUtils.Respond(c, http.StatusOK, resp, err)
}

func (h *httpController) grantTokenAccessReadAndWrite(c *gin.Context) {
	resp := &types.GrantTokenAccessResponse{}
	s, err := h.wm.GetOrCreateSession(c)
	if err != nil {
		httpUtils.Respond(c, http.StatusOK, resp, err)
		return
	}
	request := &types.GrantTokenAccessRequest{
		Session: s,
		Token:   project.AccessToken(c.Param("token")),
	}
	err = h.wm.GrantTokenAccessReadAndWrite(c, request, resp)
	if err2 := h.wm.Flush(c, s); err == nil && err2 != nil {
		err = err2
	}
	httpUtils.Respond(c, http.StatusOK, resp, err)
}

func (h *httpController) grantTokenAccessReadOnly(c *gin.Context) {
	resp := &types.GrantTokenAccessResponse{}
	s, err := h.wm.GetOrCreateSession(c)
	if err != nil {
		httpUtils.Respond(c, http.StatusOK, resp, err)
		return
	}
	request := &types.GrantTokenAccessRequest{
		Session: s,
		Token:   project.AccessToken(c.Param("token")),
	}
	err = h.wm.GrantTokenAccessReadOnly(c, request, resp)
	if err2 := h.wm.Flush(c, s); err == nil && err2 != nil {
		err = err2
	}
	httpUtils.Respond(c, http.StatusOK, resp, err)
}

func (h *httpController) addProjectToTag(c *gin.Context) {
	s, err := h.wm.GetOrCreateSession(c)
	if err != nil {
		httpUtils.Respond(c, http.StatusOK, nil, err)
		return
	}
	request := &types.AddProjectToTagRequest{
		Session:   s,
		ProjectId: httpUtils.GetId(c, "projectId"),
		TagId:     httpUtils.GetId(c, "tagId"),
	}
	err = h.wm.AddProjectToTag(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) createTag(c *gin.Context) {
	resp := &types.CreateTagResponse{}
	s, err := h.wm.GetOrCreateSession(c)
	if err != nil {
		httpUtils.Respond(c, http.StatusOK, resp, err)
		return
	}
	request := &types.CreateTagRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.Session = s
	err = h.wm.CreateTag(c, request, resp)
	httpUtils.Respond(c, http.StatusOK, resp, err)
}

func (h *httpController) deleteTag(c *gin.Context) {
	s, err := h.wm.GetOrCreateSession(c)
	if err != nil {
		httpUtils.Respond(c, http.StatusOK, nil, err)
		return
	}
	request := &types.DeleteTagRequest{
		Session: s,
		TagId:   httpUtils.GetId(c, "tagId"),
	}
	err = h.wm.DeleteTag(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) renameTag(c *gin.Context) {
	s, err := h.wm.GetOrCreateSession(c)
	if err != nil {
		httpUtils.Respond(c, http.StatusOK, nil, err)
		return
	}
	request := &types.RenameTagRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.TagId = httpUtils.GetId(c, "tagId")
	request.Session = s
	err = h.wm.RenameTag(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) removeProjectToTag(c *gin.Context) {
	s, err := h.wm.GetOrCreateSession(c)
	if err != nil {
		httpUtils.Respond(c, http.StatusOK, nil, err)
		return
	}
	request := &types.RemoveProjectToTagRequest{
		Session:   s,
		ProjectId: httpUtils.GetId(c, "projectId"),
		TagId:     httpUtils.GetId(c, "tagId"),
	}
	err = h.wm.RemoveProjectFromTag(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) getUserContacts(c *gin.Context) {
	resp := &types.GetUserContactsResponse{}
	s, err := h.wm.GetOrCreateSession(c)
	if err != nil {
		httpUtils.Respond(c, http.StatusOK, resp, err)
		return
	}
	request := &types.GetUserContactsRequest{Session: s}
	err = h.wm.GetUserContacts(c, request, resp)
	httpUtils.Respond(c, http.StatusOK, resp, err)
}

func (h *httpController) archiveProject(c *gin.Context) {
	s, err := h.wm.GetOrCreateSession(c)
	if err != nil {
		httpUtils.Respond(c, http.StatusOK, nil, err)
		return
	}
	request := &types.ArchiveProjectRequest{
		Session:   s,
		ProjectId: httpUtils.GetId(c, "projectId"),
	}
	err = h.wm.ArchiveProject(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) unArchiveProject(c *gin.Context) {
	s, err := h.wm.GetOrCreateSession(c)
	if err != nil {
		httpUtils.Respond(c, http.StatusOK, nil, err)
		return
	}
	request := &types.UnArchiveProjectRequest{
		Session:   s,
		ProjectId: httpUtils.GetId(c, "projectId"),
	}
	err = h.wm.UnArchiveProject(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) trashProject(c *gin.Context) {
	s, err := h.wm.GetOrCreateSession(c)
	if err != nil {
		httpUtils.Respond(c, http.StatusOK, nil, err)
		return
	}
	request := &types.TrashProjectRequest{
		Session:   s,
		ProjectId: httpUtils.GetId(c, "projectId"),
	}
	err = h.wm.TrashProject(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) unTrashProject(c *gin.Context) {
	s, err := h.wm.GetOrCreateSession(c)
	if err != nil {
		httpUtils.Respond(c, http.StatusOK, nil, err)
		return
	}
	request := &types.UnTrashProjectRequest{
		Session:   s,
		ProjectId: httpUtils.GetId(c, "projectId"),
	}
	err = h.wm.UnTrashProject(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

const contentTypeOctetStream = "application/octet-stream"

func (h *httpController) getProjectFile(c *gin.Context) {
	s, err := h.wm.GetOrCreateSession(c)
	if err != nil {
		httpUtils.Respond(c, http.StatusOK, nil, err)
		return
	}
	request := &types.GetProjectFileRequest{
		Session:   s,
		ProjectId: httpUtils.GetId(c, "projectId"),
		FileId:    httpUtils.GetId(c, "fileId"),
	}
	response := &types.GetProjectFileResponse{}
	err = h.wm.GetProjectFile(c, request, response)
	if err != nil {
		httpUtils.Respond(c, http.StatusOK, nil, err)
		return
	}
	cd := fmt.Sprintf("attachment; filename=%q", response.Filename)
	c.Writer.Header().Set("Content-Disposition", cd)
	c.DataFromReader(http.StatusOK, response.Size, contentTypeOctetStream, response.Reader, nil)
}

func (h *httpController) getProjectFileSize(c *gin.Context) {
	s, err := h.wm.GetOrCreateSession(c)
	if err != nil {
		httpUtils.Respond(c, http.StatusOK, nil, err)
		return
	}
	request := &types.GetProjectFileSizeRequest{
		Session:   s,
		ProjectId: httpUtils.GetId(c, "projectId"),
		FileId:    httpUtils.GetId(c, "fileId"),
	}
	response := &types.GetProjectFileSizeResponse{}
	err = h.wm.GetProjectFileSize(c, request, response)
	if err == nil {
		c.Header("Content-Length", strconv.FormatInt(response.Size, 10))
	}
	httpUtils.Respond(c, http.StatusOK, nil, err)
}
