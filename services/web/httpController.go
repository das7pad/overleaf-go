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

	"github.com/das7pad/overleaf-go/pkg/asyncForm"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/httpUtils"
	"github.com/das7pad/overleaf-go/pkg/jwt/projectJWT"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/projectInvite"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	clsiTypes "github.com/das7pad/overleaf-go/services/clsi/pkg/types"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

const (
	maxDocSize = 2 * 1024 * 1024
)

func newHttpController(wm web.Manager) httpController {
	return httpController{wm: wm}
}

type httpController struct {
	wm web.Manager
}

func (h *httpController) GetRouter(
	clientIPOptions *httpUtils.ClientIPOptions,
	corsOptions httpUtils.CORSOptions,
) *gin.Engine {
	router := httpUtils.NewRouter(&httpUtils.RouterOptions{
		StatusMessage:   "web is alive (go)\n",
		ClientIPOptions: clientIPOptions,
	})

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
	publicApiRouter.POST("/open", h.openInOverleaf)
	publicApiRouter.POST("/beta/opt-in", h.optInBetaProgram)
	publicApiRouter.POST("/beta/opt-out", h.optOutBetaProgram)
	publicApiRouter.POST("/grant/ro/:token", h.grantTokenAccessReadOnly)
	publicApiRouter.POST("/grant/rw/:token", h.grantTokenAccessReadAndWrite)
	publicApiRouter.POST("/project/new", h.createExampleProject)
	publicApiRouter.POST("/project/new/upload", h.createFromZip)
	publicApiRouter.GET("/project/download/zip", h.createMultiProjectZIP)
	publicApiRouter.GET("/user/contacts", h.getUserContacts)
	publicApiRouter.POST("/user/delete", h.deleteUser)
	publicApiRouter.POST("/user/emails/confirm", h.confirmEmail)
	publicApiRouter.POST("/user/emails/resend_confirmation", h.resendEmailConfirmation)
	publicApiRouter.POST("/user/password/reset", h.requestPasswordReset)
	publicApiRouter.POST("/user/password/set", h.setPassword)
	publicApiRouter.POST("/user/password/update", h.changePassword)
	publicApiRouter.POST("/user/sessions/clear", h.clearSessions)
	publicApiRouter.PUT("/user/settings/editor", h.updateEditorConfig)
	publicApiRouter.PUT("/user/settings/email", h.changeEmailAddress)
	publicApiRouter.PUT("/user/settings/name", h.setUserName)
	publicApiRouter.POST("/user/jwt", h.getLoggedInUserJWT)
	publicApiRouter.GET("/user/projects", h.getUserProjects)
	publicApiRouter.POST("/login", h.login)
	publicApiRouter.POST("/logout", h.logout)

	{
		// Notifications routes
		r := publicApiRouter.Group("/notifications")
		r.GET("", h.getUserNotifications)
		rById := publicApiRouter.Group("/notification/:notificationId")
		rById.Use(httpUtils.ValidateAndSetId("notificationId"))
		r.DELETE("", h.removeNotification)
	}

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
		r.DELETE("", h.deleteProject)
		r.DELETE("/archive", h.unArchiveProject)
		r.POST("/archive", h.archiveProject)
		r.POST("/clone", h.cloneProject)
		r.POST("/compile/headless", h.compileProjectHeadless)
		r.GET("/entities", h.getProjectEntities)
		r.POST("/jwt", h.getProjectJWT)
		r.POST("/leave", h.leaveProject)
		r.POST("/rename", h.renameProject)
		r.DELETE("/trash", h.unTrashProject)
		r.POST("/trash", h.trashProject)
		r.POST("/undelete", h.deleteProject)
		r.POST("/ws/bootstrap", h.getWSBootstrap)
		r.GET("/download/zip", h.createProjectZIP)

		rFile := r.Group("/file/:fileId")
		rFile.Use(httpUtils.ValidateAndSetId("fileId"))
		rFile.GET("", h.getProjectFile)
		rFile.HEAD("", h.getProjectFileSize)

		rInvite := r.Group("/invite")
		rTokenInvite := rInvite.Group("/token/:token")
		rTokenInvite.POST("/accept", h.acceptProjectInvite)
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
		// Write endpoints
		r := projectJWTRouter.Group("")
		r.Use(requireWriteAccess)

		r.PUT("/settings/compiler", h.setCompiler)
		r.PUT("/settings/imageName", h.setImageName)
		r.PUT("/settings/spellCheckLanguage", h.setSpellCheckLanguage)
		r.PUT("/settings/rootDocId", h.setRootDocId)

		r.POST("/doc", h.addDocToProject)
		r.POST("/folder", h.addFolderToProject)
		r.POST("/linked_file", h.createLinkedFile)

		rDoc := r.Group("/doc/:docId")
		rDoc.Use(httpUtils.ValidateAndSetId("docId"))
		rDoc.DELETE("", h.deleteDocFromProject)
		rDoc.POST("/rename", h.renameDocInProject)
		rDoc.POST("/move", h.moveDocInProject)
		rDoc.POST("/restore", h.restoreDeletedDocInProject)

		rDocV := rDoc.Group("/version/:version")
		rDocV.POST("/restore", h.restoreDocVersion)

		rFile := r.Group("/file/:fileId")
		rFile.Use(httpUtils.ValidateAndSetId("fileId"))
		rFile.DELETE("", h.deleteFileFromProject)
		rFile.POST("/rename", h.renameFileInProject)
		rFile.POST("/move", h.moveFileInProject)

		rLinkedFile := r.Group("/linked_file/:fileId")
		rLinkedFile.Use(httpUtils.ValidateAndSetId("fileId"))
		rLinkedFile.POST("/refresh", h.refreshLinkedFile)

		rFolder := r.Group("/folder/:folderId")
		rFolder.Use(httpUtils.ValidateAndSetId("folderId"))
		rFolder.DELETE("", h.deleteFolderFromProject)
		rFolder.POST("/rename", h.renameFolderInProject)
		rFolder.POST("/move", h.moveFolderInProject)
		rFolder.POST("/upload", h.uploadFile)
	}
	{
		// block access for token users with readOnly project access
		r := projectJWTRouter.Group("")
		r.Use(blockRestrictedUsers)
		r.GET("/members", h.listProjectMembers)
		r.GET("/messages", h.getProjectMessages)
		r.POST("/messages", h.sendProjectMessage)

		// History
		r.GET("/updates", h.getProjectHistoryUpdates)
		rDoc := r.Group("/doc/:docId")
		rDoc.Use(httpUtils.ValidateAndSetId("docId"))
		rDoc.GET("/diff", h.getProjectDocDiff)
	}
	{
		// admin endpoints
		r := projectJWTRouter.Group("")
		r.Use(requireAdminAccess)

		r.PUT("/settings/admin/publicAccessLevel", h.setPublicAccessLevel)

		r.POST("/invite", h.createProjectInvite)
		r.GET("/invites", h.listProjectInvites)
		rInvite := r.Group("/invite/:inviteId")
		rInvite.Use(httpUtils.ValidateAndSetId("inviteId"))
		rInvite.DELETE("", h.revokeProjectInvite)
		rInvite.POST("/resend", h.resendProjectInvite)

		r.POST("/transfer-ownership", h.transferProjectOwnership)
		rUser := r.Group("/users/:userId")
		rUser.Use(httpUtils.ValidateAndSetId("userId"))
		rUser.DELETE("", h.removeMemberFromProject)
		rUser.PUT("", h.setMemberPrivilegeLevelInProject)
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

func requireAdminAccess(c *gin.Context) {
	err := projectJWT.MustGet(c).PrivilegeLevel.CheckIsAtLeast(
		sharedTypes.PrivilegeLevelOwner,
	)
	if err != nil {
		httpUtils.Respond(c, http.StatusOK, nil, err)
		return
	}
}

func requireWriteAccess(c *gin.Context) {
	err := projectJWT.MustGet(c).PrivilegeLevel.CheckIsAtLeast(
		sharedTypes.PrivilegeLevelReadAndWrite,
	)
	if err != nil {
		httpUtils.Respond(c, http.StatusOK, nil, err)
		return
	}
}

func mustGetSignedCompileProjectOptionsFromJwt(c *gin.Context) types.SignedCompileProjectRequestOptions {
	return projectJWT.MustGet(c).SignedCompileProjectRequestOptions
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
		c.Request.Context(),
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
		c.Request.Context(),
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
		c.Request.Context(),
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
		c.Request.Context(),
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
		c.Request.Context(),
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
	err := h.wm.LoadEditor(c.Request.Context(), request, response)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) projectListLocals(c *gin.Context) {
	request := &types.ProjectListRequest{
		UserId: httpUtils.GetId(c, "userId"),
	}
	response := &types.ProjectListResponse{}
	err := h.wm.ProjectList(c.Request.Context(), request, response)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) getSystemMessages(c *gin.Context) {
	m := h.wm.GetAllCached(
		c.Request.Context(), httpUtils.GetId(c, "userId"),
	)
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
	err = h.wm.GetUserProjects(c.Request.Context(), request, resp)
	httpUtils.Respond(c, http.StatusOK, resp, err)
}

func (h *httpController) getMetadataForProject(c *gin.Context) {
	projectId := mustGetSignedCompileProjectOptionsFromJwt(c).ProjectId
	resp, err := h.wm.GetMetadataForProject(c.Request.Context(), projectId)
	httpUtils.Respond(c, http.StatusOK, resp, err)
}

func (h *httpController) getMetadataForDoc(c *gin.Context) {
	projectId := mustGetSignedCompileProjectOptionsFromJwt(c).ProjectId
	docId := httpUtils.GetId(c, "docId")
	request := &types.ProjectDocMetadataRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	resp, err := h.wm.GetMetadataForDoc(
		c.Request.Context(), projectId, docId, request,
	)
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
	err = h.wm.Login(c.Request.Context(), request, resp)
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
	err = h.wm.Logout(c.Request.Context(), request)
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
	err = h.wm.GetLoggedInUserJWT(c.Request.Context(), request, &resp)
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
	err = h.wm.GetProjectJWT(c.Request.Context(), request, &resp)
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
	err = h.wm.GetWSBootstrap(c.Request.Context(), request, &resp)
	httpUtils.Respond(c, http.StatusOK, resp, err)
}

func (h *httpController) getProjectMessages(c *gin.Context) {
	request := &types.GetProjectChatMessagesRequest{}
	if err := c.MustBindWith(request, binding.Query); err != nil {
		return
	}
	request.ProjectId = projectJWT.MustGet(c).ProjectId
	response := &types.GetProjectChatMessagesResponse{}
	err := h.wm.GetProjectMessages(c.Request.Context(), request, response)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) sendProjectMessage(c *gin.Context) {
	request := &types.SendProjectChatMessageRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.ProjectId = projectJWT.MustGet(c).ProjectId
	request.UserId = projectJWT.MustGet(c).UserId
	err := h.wm.SendProjectMessage(c.Request.Context(), request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) optInBetaProgram(c *gin.Context) {
	s, err := h.wm.GetOrCreateSession(c)
	if err != nil {
		httpUtils.Respond(c, http.StatusOK, nil, err)
		return
	}
	request := &types.OptInBetaProgramRequest{Session: s}
	err = h.wm.OptInBetaProgram(c.Request.Context(), request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) optOutBetaProgram(c *gin.Context) {
	s, err := h.wm.GetOrCreateSession(c)
	if err != nil {
		httpUtils.Respond(c, http.StatusOK, nil, err)
		return
	}
	request := &types.OptOutBetaProgramRequest{Session: s}
	err = h.wm.OptOutBetaProgram(c.Request.Context(), request)
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
	err = h.wm.GetProjectEntities(c.Request.Context(), request, resp)
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
	err = h.wm.GrantTokenAccessReadAndWrite(c.Request.Context(), request, resp)
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
	err = h.wm.GrantTokenAccessReadOnly(c.Request.Context(), request, resp)
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
	err = h.wm.AddProjectToTag(c.Request.Context(), request)
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
	err = h.wm.CreateTag(c.Request.Context(), request, resp)
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
	err = h.wm.DeleteTag(c.Request.Context(), request)
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
	err = h.wm.RenameTag(c.Request.Context(), request)
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
	err = h.wm.RemoveProjectFromTag(c.Request.Context(), request)
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
	err = h.wm.GetUserContacts(c.Request.Context(), request, resp)
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
	err = h.wm.ArchiveProject(c.Request.Context(), request)
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
	err = h.wm.UnArchiveProject(c.Request.Context(), request)
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
	err = h.wm.TrashProject(c.Request.Context(), request)
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
	err = h.wm.UnTrashProject(c.Request.Context(), request)
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
	err = h.wm.GetProjectFile(c.Request.Context(), request, response)
	if err != nil {
		httpUtils.Respond(c, http.StatusOK, nil, err)
		return
	}
	cd := fmt.Sprintf("attachment; filename=%q", response.Filename)
	c.Header("Content-Disposition", cd)
	httpUtils.EndTotalTimer(c)
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
	err = h.wm.GetProjectFileSize(c.Request.Context(), request, response)
	if err == nil {
		c.Header("Content-Length", strconv.FormatInt(response.Size, 10))
	}
	httpUtils.Respond(c, http.StatusOK, nil, err)
}

func (h *httpController) addDocToProject(c *gin.Context) {
	o := mustGetSignedCompileProjectOptionsFromJwt(c)
	request := &types.AddDocRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.ProjectId = o.ProjectId
	response := &types.AddDocResponse{}
	err := h.wm.AddDocToProject(c.Request.Context(), request, response)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) addFolderToProject(c *gin.Context) {
	o := mustGetSignedCompileProjectOptionsFromJwt(c)
	request := &types.AddFolderRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.ProjectId = o.ProjectId
	response := &types.AddFolderResponse{}
	err := h.wm.AddFolderToProject(c.Request.Context(), request, response)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) uploadFile(c *gin.Context) {
	j := projectJWT.MustGet(c)
	d := &httpUtils.UploadDetails{}
	if !httpUtils.ProcessFileUpload(c, types.MaxUploadSize, maxDocSize, d) {
		return
	}
	request := &types.UploadFileRequest{
		ProjectId:      j.ProjectId,
		UserId:         j.UserId,
		ParentFolderId: httpUtils.GetId(c, "folderId"),
		UploadDetails: types.UploadDetails{
			File:     d.File,
			FileName: d.FileName,
			Size:     d.Size,
		},
	}
	err := h.wm.UploadFile(c.Request.Context(), request)
	_ = d.File.Close()
	httpUtils.Respond(c, http.StatusOK, asyncForm.Response{}, err)
}

func (h *httpController) deleteDocFromProject(c *gin.Context) {
	o := mustGetSignedCompileProjectOptionsFromJwt(c)
	request := &types.DeleteDocRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.ProjectId = o.ProjectId
	request.DocId = httpUtils.GetId(c, "docId")
	err := h.wm.DeleteDocFromProject(c.Request.Context(), request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) deleteFileFromProject(c *gin.Context) {
	o := mustGetSignedCompileProjectOptionsFromJwt(c)
	request := &types.DeleteFileRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.ProjectId = o.ProjectId
	request.FileId = httpUtils.GetId(c, "fileId")
	err := h.wm.DeleteFileFromProject(c.Request.Context(), request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) deleteFolderFromProject(c *gin.Context) {
	o := mustGetSignedCompileProjectOptionsFromJwt(c)
	request := &types.DeleteFolderRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.ProjectId = o.ProjectId
	request.FolderId = httpUtils.GetId(c, "folderId")
	err := h.wm.DeleteFolderFromProject(c.Request.Context(), request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) moveDocInProject(c *gin.Context) {
	o := mustGetSignedCompileProjectOptionsFromJwt(c)
	request := &types.MoveDocRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.ProjectId = o.ProjectId
	request.DocId = httpUtils.GetId(c, "docId")
	err := h.wm.MoveDocInProject(c.Request.Context(), request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) moveFileInProject(c *gin.Context) {
	o := mustGetSignedCompileProjectOptionsFromJwt(c)
	request := &types.MoveFileRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.ProjectId = o.ProjectId
	request.FileId = httpUtils.GetId(c, "fileId")
	err := h.wm.MoveFileInProject(c.Request.Context(), request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) moveFolderInProject(c *gin.Context) {
	o := mustGetSignedCompileProjectOptionsFromJwt(c)
	request := &types.MoveFolderRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.ProjectId = o.ProjectId
	request.FolderId = httpUtils.GetId(c, "folderId")
	err := h.wm.MoveFolderInProject(c.Request.Context(), request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) renameDocInProject(c *gin.Context) {
	o := mustGetSignedCompileProjectOptionsFromJwt(c)
	request := &types.RenameDocRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.ProjectId = o.ProjectId
	request.DocId = httpUtils.GetId(c, "docId")
	err := h.wm.RenameDocInProject(c.Request.Context(), request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) renameFileInProject(c *gin.Context) {
	o := mustGetSignedCompileProjectOptionsFromJwt(c)
	request := &types.RenameFileRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.ProjectId = o.ProjectId
	request.FileId = httpUtils.GetId(c, "fileId")
	err := h.wm.RenameFileInProject(c.Request.Context(), request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) renameFolderInProject(c *gin.Context) {
	o := mustGetSignedCompileProjectOptionsFromJwt(c)
	request := &types.RenameFolderRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.ProjectId = o.ProjectId
	request.FolderId = httpUtils.GetId(c, "folderId")
	err := h.wm.RenameFolderInProject(c.Request.Context(), request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) restoreDeletedDocInProject(c *gin.Context) {
	o := mustGetSignedCompileProjectOptionsFromJwt(c)
	request := &types.RestoreDeletedDocRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.ProjectId = o.ProjectId
	request.DocId = httpUtils.GetId(c, "docId")
	response := &types.RestoreDeletedDocResponse{}
	err := h.wm.RestoreDeletedDocInProject(c.Request.Context(), request, response)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) renameProject(c *gin.Context) {
	s, err := h.wm.GetOrCreateSession(c)
	if err != nil {
		httpUtils.Respond(c, http.StatusOK, nil, err)
		return
	}
	request := &types.RenameProjectRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.Session = s
	request.ProjectId = httpUtils.GetId(c, "projectId")
	err = h.wm.RenameProject(c.Request.Context(), request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) acceptProjectInvite(c *gin.Context) {
	response := &types.AcceptProjectInviteResponse{}
	s, err := h.wm.GetOrCreateSession(c)
	if err != nil {
		httpUtils.Respond(c, http.StatusOK, response, err)
		return
	}
	request := &types.AcceptProjectInviteRequest{
		Session:   s,
		ProjectId: httpUtils.GetId(c, "projectId"),
		Token:     projectInvite.Token(c.Param("token")),
	}
	err = h.wm.AcceptProjectInvite(c.Request.Context(), request, response)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) createProjectInvite(c *gin.Context) {
	o := mustGetSignedCompileProjectOptionsFromJwt(c)
	request := &types.CreateProjectInviteRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.ProjectId = o.ProjectId
	request.SenderUserId = o.UserId
	err := h.wm.CreateProjectInvite(c.Request.Context(), request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) resendProjectInvite(c *gin.Context) {
	o := mustGetSignedCompileProjectOptionsFromJwt(c)
	request := &types.ResendProjectInviteRequest{
		ProjectId: o.ProjectId,
		InviteId:  httpUtils.GetId(c, "inviteId"),
	}
	err := h.wm.ResendProjectInvite(c.Request.Context(), request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) revokeProjectInvite(c *gin.Context) {
	o := mustGetSignedCompileProjectOptionsFromJwt(c)
	request := &types.RevokeProjectInviteRequest{
		ProjectId: o.ProjectId,
		InviteId:  httpUtils.GetId(c, "inviteId"),
	}
	err := h.wm.RevokeProjectInvite(c.Request.Context(), request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) listProjectInvites(c *gin.Context) {
	o := mustGetSignedCompileProjectOptionsFromJwt(c)
	request := &types.ListProjectInvitesRequest{
		ProjectId: o.ProjectId,
	}
	response := &types.ListProjectInvitesResponse{}
	err := h.wm.ListProjectInvites(c.Request.Context(), request, response)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) listProjectMembers(c *gin.Context) {
	o := mustGetSignedCompileProjectOptionsFromJwt(c)
	request := &types.ListProjectMembersRequest{
		ProjectId: o.ProjectId,
	}
	response := &types.ListProjectMembersResponse{}
	err := h.wm.ListProjectMembers(c.Request.Context(), request, response)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) removeMemberFromProject(c *gin.Context) {
	o := mustGetSignedCompileProjectOptionsFromJwt(c)
	request := &types.RemoveProjectMemberRequest{
		ProjectId: o.ProjectId,
		Epoch:     projectJWT.MustGet(c).Epoch,
		UserId:    httpUtils.GetId(c, "userId"),
	}
	err := h.wm.RemoveMemberFromProject(c.Request.Context(), request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) setMemberPrivilegeLevelInProject(c *gin.Context) {
	o := mustGetSignedCompileProjectOptionsFromJwt(c)
	request := &types.SetMemberPrivilegeLevelInProjectRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.ProjectId = o.ProjectId
	request.UserId = httpUtils.GetId(c, "userId")
	err := h.wm.SetMemberPrivilegeLevelInProject(c.Request.Context(), request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) transferProjectOwnership(c *gin.Context) {
	o := mustGetSignedCompileProjectOptionsFromJwt(c)
	request := &types.TransferProjectOwnershipRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.ProjectId = o.ProjectId
	request.PreviousOwnerId = o.UserId
	err := h.wm.TransferProjectOwnership(c.Request.Context(), request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) leaveProject(c *gin.Context) {
	s, err := h.wm.GetOrCreateSession(c)
	if err != nil {
		httpUtils.Respond(c, http.StatusOK, nil, err)
		return
	}
	request := &types.LeaveProjectRequest{
		Session:   s,
		ProjectId: httpUtils.GetId(c, "projectId"),
	}
	err = h.wm.LeaveProject(c.Request.Context(), request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) setCompiler(c *gin.Context) {
	o := mustGetSignedCompileProjectOptionsFromJwt(c)
	request := &types.SetCompilerRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.ProjectId = o.ProjectId
	err := h.wm.SetCompiler(c.Request.Context(), request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) setImageName(c *gin.Context) {
	o := mustGetSignedCompileProjectOptionsFromJwt(c)
	request := &types.SetImageNameRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.ProjectId = o.ProjectId
	err := h.wm.SetImageName(c.Request.Context(), request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) setSpellCheckLanguage(c *gin.Context) {
	o := mustGetSignedCompileProjectOptionsFromJwt(c)
	request := &types.SetSpellCheckLanguageRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.ProjectId = o.ProjectId
	err := h.wm.SetSpellCheckLanguage(c.Request.Context(), request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) setRootDocId(c *gin.Context) {
	o := mustGetSignedCompileProjectOptionsFromJwt(c)
	request := &types.SetRootDocIdRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.ProjectId = o.ProjectId
	err := h.wm.SetRootDocId(c.Request.Context(), request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) setPublicAccessLevel(c *gin.Context) {
	o := mustGetSignedCompileProjectOptionsFromJwt(c)
	request := &types.SetPublicAccessLevelRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.ProjectId = o.ProjectId
	request.Epoch = projectJWT.MustGet(c).Epoch
	err := h.wm.SetPublicAccessLevel(c.Request.Context(), request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) clearSessions(c *gin.Context) {
	s, err := h.wm.GetOrCreateSession(c)
	if err != nil {
		httpUtils.Respond(c, http.StatusOK, nil, err)
		return
	}
	request := &types.ClearSessionsRequest{
		Session:   s,
		IPAddress: c.ClientIP(),
	}
	err = h.wm.ClearSessions(c.Request.Context(), request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) cloneProject(c *gin.Context) {
	s, err := h.wm.RequireLoggedInSession(c)
	if err != nil {
		httpUtils.RespondErr(c, err)
		return
	}
	request := &types.CloneProjectRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.Session = s
	request.ProjectId = httpUtils.GetId(c, "projectId")
	response := &types.CloneProjectResponse{}
	err = h.wm.CloneProject(c.Request.Context(), request, response)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) createExampleProject(c *gin.Context) {
	s, err := h.wm.RequireLoggedInSession(c)
	if err != nil {
		httpUtils.RespondErr(c, err)
		return
	}
	request := &types.CreateExampleProjectRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.Session = s
	response := &types.CreateExampleProjectResponse{}
	err = h.wm.CreateExampleProject(c.Request.Context(), request, response)
	if err != nil {
		if errors.IsValidationError(err) {
			response.Error = "Error: " + err.Error()
		}
	}
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) createFromZip(c *gin.Context) {
	s, err := h.wm.RequireLoggedInSession(c)
	if err != nil {
		httpUtils.RespondErr(c, err)
		return
	}

	d := &httpUtils.UploadDetails{}
	if !httpUtils.ProcessFileUpload(c, types.MaxUploadSize, maxDocSize, d) {
		return
	}
	request := &types.CreateProjectFromZipRequest{
		Session: s,
		UploadDetails: types.UploadDetails{
			File:     d.File,
			FileName: d.FileName,
			Size:     d.Size,
		},
	}
	response := &types.CreateProjectResponse{}
	err = h.wm.CreateFromZip(c.Request.Context(), request, response)
	_ = d.File.Close()
	if err != nil {
		if errors.IsValidationError(err) {
			response.Error = "Error: " + err.Error()
		}
	}
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) getUserNotifications(c *gin.Context) {
	s, err := h.wm.RequireLoggedInSession(c)
	if err != nil {
		httpUtils.RespondErr(c, err)
		return
	}
	request := &types.GetNotificationsRequest{
		Session: s,
	}
	response := &types.GetNotificationsResponse{}
	err = h.wm.GetUserNotifications(c.Request.Context(), request, response)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) removeNotification(c *gin.Context) {
	s, err := h.wm.RequireLoggedInSession(c)
	if err != nil {
		httpUtils.RespondErr(c, err)
		return
	}
	request := &types.RemoveNotificationRequest{
		Session:        s,
		NotificationId: httpUtils.GetId(c, "notificationId"),
	}
	err = h.wm.RemoveNotification(c.Request.Context(), request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) openInOverleaf(c *gin.Context) {
	s, err := h.wm.RequireLoggedInSession(c)
	if err != nil {
		httpUtils.RespondErr(c, err)
		return
	}
	request := &types.OpenInOverleafRequest{}
	if c.ContentType() == "application/json" {
		if !httpUtils.MustParseJSON(request, c) {
			return
		}
	} else {
		if err = c.Request.ParseMultipartForm(0); err != nil {
			httpUtils.RespondErr(c, &errors.ValidationError{Msg: err.Error()})
			return
		}
		if err = request.PopulateFromParams(c.Request.Form); err != nil {
			httpUtils.RespondErr(c, err)
			return
		}
	}
	request.Session = s
	response := &types.CreateProjectResponse{}
	err = h.wm.OpenInOverleaf(c.Request.Context(), request, response)
	if err != nil && errors.IsValidationError(err) {
		response.Error = err.Error()
	}
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) compileProjectHeadless(c *gin.Context) {
	s, err := h.wm.RequireLoggedInSession(c)
	if err != nil {
		httpUtils.RespondErr(c, err)
		return
	}
	request := &types.CompileProjectHeadlessRequest{
		ProjectId: httpUtils.GetId(c, "projectId"),
		UserId:    s.User.Id,
	}
	response := &types.CompileProjectResponse{}
	err = h.wm.CompileHeadLess(
		c.Request.Context(),
		request,
		response,
	)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) createLinkedFile(c *gin.Context) {
	o := mustGetSignedCompileProjectOptionsFromJwt(c)
	request := &types.CreateLinkedFileRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.UserId = o.UserId
	request.ProjectId = o.ProjectId
	err := h.wm.CreateLinkedFile(c.Request.Context(), request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) refreshLinkedFile(c *gin.Context) {
	o := mustGetSignedCompileProjectOptionsFromJwt(c)
	request := &types.RefreshLinkedFileRequest{
		UserId:    o.UserId,
		ProjectId: o.ProjectId,
		FileId:    httpUtils.GetId(c, "fileId"),
	}
	err := h.wm.RefreshLinkedFile(c.Request.Context(), request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) createProjectZIP(c *gin.Context) {
	s, err := h.wm.GetOrCreateSession(c)
	if err != nil {
		httpUtils.RespondErr(c, err)
		return
	}
	request := &types.CreateProjectZIPRequest{
		Session:   s,
		ProjectId: httpUtils.GetId(c, "projectId"),
	}
	response := &types.CreateProjectZIPResponse{}
	defer response.Cleanup()

	err = h.wm.CreateProjectZIP(c.Request.Context(), request, response)
	if err != nil {
		httpUtils.RespondErr(c, err)
		return
	}
	cd := fmt.Sprintf("attachment; filename=%q", response.Filename)
	c.Header("Content-Disposition", cd)
	httpUtils.EndTotalTimer(c)
	http.ServeFile(c.Writer, c.Request, response.FSPath)
}

func (h *httpController) createMultiProjectZIP(c *gin.Context) {
	s, err := h.wm.RequireLoggedInSession(c)
	if err != nil {
		httpUtils.RespondErr(c, err)
		return
	}
	request := &types.CreateMultiProjectZIPRequest{}
	if err = request.ParseProjectIds(c.Query("project_ids")); err != nil {
		httpUtils.RespondErr(c, err)
		return
	}
	request.Session = s
	response := &types.CreateProjectZIPResponse{}
	defer response.Cleanup()

	err = h.wm.CreateMultiProjectZIP(c.Request.Context(), request, response)
	if err != nil {
		httpUtils.RespondErr(c, err)
		return
	}
	cd := fmt.Sprintf("attachment; filename=%q", response.Filename)
	c.Header("Content-Disposition", cd)
	httpUtils.EndTotalTimer(c)
	http.ServeFile(c.Writer, c.Request, response.FSPath)
}

func (h *httpController) deleteProject(c *gin.Context) {
	s, err := h.wm.RequireLoggedInSession(c)
	if err != nil {
		httpUtils.RespondErr(c, err)
		return
	}
	request := &types.DeleteProjectRequest{
		Session:   s,
		ProjectId: httpUtils.GetId(c, "projectId"),
		IPAddress: c.ClientIP(),
	}
	err = h.wm.DeleteProject(c.Request.Context(), request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) undeleteProject(c *gin.Context) {
	s, err := h.wm.RequireLoggedInSession(c)
	if err != nil {
		httpUtils.RespondErr(c, err)
		return
	}
	request := &types.UnDeleteProjectRequest{
		Session:   s,
		ProjectId: httpUtils.GetId(c, "projectId"),
	}
	err = h.wm.UnDeleteProject(c.Request.Context(), request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) deleteUser(c *gin.Context) {
	s, err := h.wm.RequireLoggedInSession(c)
	if err != nil {
		httpUtils.RespondErr(c, err)
		return
	}
	request := &types.DeleteUserRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.Session = s
	request.IPAddress = c.ClientIP()
	err = h.wm.DeleteUser(c.Request.Context(), request)
	_ = h.wm.Flush(c, s)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) updateEditorConfig(c *gin.Context) {
	s, err := h.wm.RequireLoggedInSession(c)
	if err != nil {
		httpUtils.RespondErr(c, err)
		return
	}
	request := &types.UpdateEditorConfigRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.Session = s
	err = h.wm.UpdateEditorConfig(c.Request.Context(), request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) changeEmailAddress(c *gin.Context) {
	s, err := h.wm.RequireLoggedInSession(c)
	if err != nil {
		httpUtils.RespondErr(c, err)
		return
	}
	request := &types.ChangeEmailAddressRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.Session = s
	request.IPAddress = c.ClientIP()
	err = h.wm.ChangeEmailAddress(c.Request.Context(), request)
	_ = h.wm.Flush(c, s)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) setUserName(c *gin.Context) {
	s, err := h.wm.RequireLoggedInSession(c)
	if err != nil {
		httpUtils.RespondErr(c, err)
		return
	}
	request := &types.SetUserName{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.Session = s
	err = h.wm.SetUserName(c.Request.Context(), request)
	_ = h.wm.Flush(c, s)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) changePassword(c *gin.Context) {
	s, err := h.wm.RequireLoggedInSession(c)
	if err != nil {
		httpUtils.RespondErr(c, err)
		return
	}
	request := &types.ChangePasswordRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.Session = s
	request.IPAddress = c.ClientIP()
	res := &types.ChangePasswordResponse{}
	err = h.wm.ChangePassword(c.Request.Context(), request, res)
	httpUtils.Respond(c, http.StatusOK, res, err)
}

func (h *httpController) requestPasswordReset(c *gin.Context) {
	request := &types.RequestPasswordResetRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	err := h.wm.RequestPasswordReset(c.Request.Context(), request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) setPassword(c *gin.Context) {
	request := &types.SetPasswordRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.IPAddress = c.ClientIP()
	err := h.wm.SetPassword(c.Request.Context(), request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) confirmEmail(c *gin.Context) {
	request := &types.ConfirmEmailRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	err := h.wm.ConfirmEmail(c.Request.Context(), request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) resendEmailConfirmation(c *gin.Context) {
	s, err := h.wm.RequireLoggedInSession(c)
	if err != nil {
		httpUtils.RespondErr(c, err)
		return
	}
	request := &types.ResendEmailConfirmationRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.Session = s
	err = h.wm.ResendEmailConfirmation(c.Request.Context(), request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) getProjectHistoryUpdates(c *gin.Context) {
	o := mustGetSignedCompileProjectOptionsFromJwt(c)
	request := &types.GetProjectHistoryUpdatesRequest{}
	if err := c.MustBindWith(request, binding.Query); err != nil {
		return
	}
	request.ProjectId = o.ProjectId
	request.UserId = o.UserId
	res := &types.GetProjectHistoryUpdatesResponse{}
	err := h.wm.GetProjectHistoryUpdates(c.Request.Context(), request, res)
	httpUtils.Respond(c, http.StatusOK, res, err)
}

func (h *httpController) getProjectDocDiff(c *gin.Context) {
	o := mustGetSignedCompileProjectOptionsFromJwt(c)
	request := &types.GetDocDiffRequest{}
	if err := c.MustBindWith(request, binding.Query); err != nil {
		return
	}
	request.ProjectId = o.ProjectId
	request.DocId = httpUtils.GetId(c, "docId")
	request.UserId = o.UserId
	res := &types.GetDocDiffResponse{}
	err := h.wm.GetDocDiff(c.Request.Context(), request, res)
	httpUtils.Respond(c, http.StatusOK, res, err)
}

func (h *httpController) restoreDocVersion(c *gin.Context) {
	o := mustGetSignedCompileProjectOptionsFromJwt(c)
	i, err := strconv.ParseInt(c.Param("version"), 10, 64)
	if err != nil {
		httpUtils.RespondErr(c, &errors.ValidationError{Msg: err.Error()})
		return
	}
	request := &types.RestoreDocVersionRequest{
		ProjectId: o.ProjectId,
		DocId:     httpUtils.GetId(c, "docId"),
		UserId:    o.UserId,
		FromV:     sharedTypes.Version(i),
	}
	err = h.wm.RestoreDocVersion(c.Request.Context(), request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}
