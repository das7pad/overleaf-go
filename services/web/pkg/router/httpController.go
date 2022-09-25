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

package router

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/das7pad/overleaf-go/pkg/asyncForm"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/httpUtils"
	"github.com/das7pad/overleaf-go/pkg/jwt/projectJWT"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/models/projectInvite"
	"github.com/das7pad/overleaf-go/pkg/session"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/pkg/templates"
	clsiTypes "github.com/das7pad/overleaf-go/services/clsi/pkg/types"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

const (
	maxDocSize = sharedTypes.MaxDocSizeBytes
)

func New(wm web.Manager, corsOptions httpUtils.CORSOptions) *httpUtils.Router {
	router := httpUtils.NewRouter(&httpUtils.RouterOptions{})
	Add(router, wm, corsOptions)
	return router
}

func Add(r *httpUtils.Router, wm web.Manager, corsOptions httpUtils.CORSOptions) {
	(&httpController{
		ps: wm.GetPublicSettings(),
		wm: wm,
	}).addRoutes(r, corsOptions)
}

type httpController struct {
	ps *templates.PublicSettings
	wm web.Manager
}

func (h *httpController) addRoutes(
	router *httpUtils.Router,
	corsOptions httpUtils.CORSOptions,
) *httpUtils.Router {
	router = router.Group("")
	{
		// SECURITY: Attach gateway page before CORS middleware.
		//           All 3rd parties are allowed to send users to the gw page.
		router.GET("/docs", h.openInOverleafGatewayPage)
		router.POST("/docs", h.openInOverleafGatewayPage)
	}
	router.Use(httpUtils.CORS(corsOptions))
	router.NoRoute(h.notFound)

	{
		r := router.Group("")
		r.Use(h.blockUnsupportedBrowser)
		r.GET("/", h.homePage)
		r.GET("/admin", h.adminManageSitePage)
		r.GET("/admin/register", h.adminRegisterUsersPage)
		r.GET("/beta/participate", h.betaProgramParticipatePage)
		r.GET("/devs", h.openInOverleafDocumentationPage)
		r.GET("/health_check", h.smokeTestFull)
		r.HEAD("/health_check", h.smokeTestFull)
		r.GET("/health_check/api", h.smokeTestAPI)
		r.HEAD("/health_check/api", h.smokeTestAPI)
		r.GET("/health_check/full", h.smokeTestFull)
		r.HEAD("/health_check/full", h.smokeTestFull)
		// NOTE: Intercept cleanup of trailing slash. We might need to redirect
		//        somewhere else again and can shortcut a chain of redirects.
		r.GET("/learn", h.learn)
		r.GET("/learn/", h.learn)
		r.GET("/learn/{section}", h.learn)
		r.GET("/learn/{section}/", h.learn)
		r.GET("/learn/{section}/{page}", h.learn)
		r.GET("/learn/{section}/{page}/", h.learn)
		r.GET("/learn-scripts/images/{a}/{b}/{c}", h.proxyLearnImage)
		r.GET("/login", h.loginPage)
		r.GET("/logout", h.logoutPage)
		r.GET("/project", h.projectListPage)
		r.GET("/register", h.registerUserPage)
		r.GET("/restricted", h.restrictedPage)
		r.GET("/switch-language", h.switchLanguage)
		r.GET("/user/activate", h.activateUserPage)
		r.GET("/user/emails/confirm", h.confirmEmailPage)
		r.GET("/user/password/reset", h.requestPasswordResetPage)
		r.GET("/user/password/set", h.setPasswordPage)
		r.GET("/user/reconfirm", h.reconfirmAccountPage)
		r.GET("/user/sessions", h.sessionsPage)
		r.GET("/user/settings", h.settingsPage)
		r.GET("/read/{token}", h.tokenAccessPage)
		r.GET("/{token}", h.tokenAccessPage)

		rp := r.Group("/project/{projectId}")
		rp.GET("", h.projectEditorPage)
		rp.GET("/invite/token/{token}", h.viewProjectInvitePage)
	}

	apiRouter := router.Group("/api")
	apiRouter.Use(h.addApiCSPmw)
	apiRouter.POST("/open", h.openInOverleaf)
	apiRouter.POST("/beta/opt-in", h.optInBetaProgram)
	apiRouter.POST("/beta/opt-out", h.optOutBetaProgram)
	apiRouter.POST("/grant/ro/{token}", h.grantTokenAccessReadOnly)
	apiRouter.POST("/grant/rw/{token}", h.grantTokenAccessReadAndWrite)
	apiRouter.POST("/project/new", h.createExampleProject)
	apiRouter.POST("/project/new/upload", h.createFromZip)
	apiRouter.GET("/project/download/zip", h.createMultiProjectZIP)
	apiRouter.POST("/register", h.registerUser)
	apiRouter.GET("/spelling/dict", h.getDictionary)
	apiRouter.POST("/spelling/learn", h.learnWord)
	apiRouter.GET("/user/contacts", h.getUserContacts)
	apiRouter.POST("/user/delete", h.deleteUser)
	apiRouter.POST("/user/emails/confirm", h.confirmEmail)
	apiRouter.POST("/user/emails/resend_confirmation", h.resendEmailConfirmation)
	apiRouter.POST("/user/password/reset", h.requestPasswordReset)
	apiRouter.POST("/user/password/set", h.setPassword)
	apiRouter.POST("/user/password/update", h.changePassword)
	apiRouter.POST("/user/reconfirm", h.requestPasswordReset)
	apiRouter.POST("/user/sessions/clear", h.clearSessions)
	apiRouter.PUT("/user/settings/editor", h.updateEditorConfig)
	apiRouter.PUT("/user/settings/email", h.changeEmailAddress)
	apiRouter.PUT("/user/settings/name", h.setUserName)
	apiRouter.GET("/user/jwt", h.getLoggedInUserJWT)
	apiRouter.GET("/user/projects", h.getUserProjects)
	apiRouter.POST("/login", h.login)
	apiRouter.POST("/logout", h.logout)

	{
		// admin endpoints
		r := apiRouter.Group("/admin")
		r.POST("/register", h.adminCreateUser)
	}
	{
		// Notifications routes
		r := apiRouter.Group("/notifications")
		r.GET("", h.getUserNotifications)

		rById := apiRouter.Group("/notification/{notificationId}")
		rById.Use(httpUtils.ValidateAndSetId("notificationId"))
		rById.DELETE("", h.removeNotification)
	}
	{
		// Tag routes
		r := apiRouter.Group("/tag")
		r.POST("", h.createTag)

		rt := r.Group("/{tagId}")
		rt.Use(httpUtils.ValidateAndSetId("tagId"))
		rt.DELETE("", h.deleteTag)
		rt.POST("/rename", h.renameTag)

		rtp := rt.Group("/project/{projectId}")
		rtp.Use(httpUtils.ValidateAndSetId("projectId"))
		rtp.DELETE("", h.removeProjectToTag)
		rtp.POST("", h.addProjectToTag)
	}

	{
		// Project routes with session auth
		r := apiRouter.Group("/project/{projectId}")
		r.Use(httpUtils.ValidateAndSetId("projectId"))
		r.DELETE("", h.deleteProject)
		r.DELETE("/archive", h.unArchiveProject)
		r.POST("/archive", h.archiveProject)
		r.POST("/clone", h.cloneProject)
		r.POST("/compile/headless", h.compileProjectHeadless)
		r.GET("/entities", h.getProjectEntities)
		r.GET("/jwt", h.getProjectJWT)
		r.POST("/leave", h.leaveProject)
		r.POST("/rename", h.renameProject)
		r.DELETE("/trash", h.unTrashProject)
		r.POST("/trash", h.trashProject)
		r.POST("/undelete", h.deleteProject)
		r.GET("/ws/bootstrap", h.getWSBootstrap)
		r.GET("/download/zip", h.createProjectZIP)

		rFile := r.Group("/file/{fileId}")
		rFile.Use(httpUtils.ValidateAndSetId("fileId"))
		rFile.GET("", h.getProjectFile)
		rFile.HEAD("", h.getProjectFileSize)

		rInvite := r.Group("/invite")
		rTokenInvite := rInvite.Group("/token/{token}")
		rTokenInvite.POST("/accept", h.acceptProjectInvite)
	}

	jwtRouter := router.Group("/jwt/web")
	jwtRouter.Use(h.addApiCSPmw)

	{
		// The /system/messages endpoint is polled from both the project list
		//  and project editor pages.
		// Use a cheap authentication mechanism for this high volume traffic.
		r := jwtRouter.Group("")
		j := h.wm.GetLoggedInUserJWTHandler()
		r.Use(httpUtils.NewJWTHandler(j).Middleware())
		r.GET("/system/messages", h.getSystemMessages)
	}

	projectJWTRouter := jwtRouter.Group("/project/{projectId}")
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

		rDoc := r.Group("/doc/{docId}")
		rDoc.Use(httpUtils.ValidateAndSetId("docId"))
		rDoc.DELETE("", h.deleteDocFromProject)
		rDoc.POST("/rename", h.renameDocInProject)
		rDoc.POST("/move", h.moveDocInProject)
		rDoc.POST("/restore", h.restoreDeletedDocInProject)

		rDocV := rDoc.Group("/version/{version}")
		rDocV.POST("/restore", h.restoreDocVersion)

		rFile := r.Group("/file/{fileId}")
		rFile.Use(httpUtils.ValidateAndSetId("fileId"))
		rFile.DELETE("", h.deleteFileFromProject)
		rFile.POST("/rename", h.renameFileInProject)
		rFile.POST("/move", h.moveFileInProject)

		rLinkedFile := r.Group("/linked_file/{fileId}")
		rLinkedFile.Use(httpUtils.ValidateAndSetId("fileId"))
		rLinkedFile.POST("/refresh", h.refreshLinkedFile)

		rFolder := r.Group("/folder/{folderId}")
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
		rDoc := r.Group("/doc/{docId}")
		rDoc.Use(httpUtils.ValidateAndSetId("docId"))
		rDoc.GET("/diff", h.getProjectDocDiff)
	}
	{
		// project admin endpoints
		r := projectJWTRouter.Group("")
		r.Use(requireProjectAdminAccess)

		r.PUT("/settings/admin/publicAccessLevel", h.setPublicAccessLevel)

		r.POST("/invite", h.createProjectInvite)
		r.GET("/invites", h.listProjectInvites)
		rInvite := r.Group("/invite/{inviteId}")
		rInvite.Use(httpUtils.ValidateAndSetId("inviteId"))
		rInvite.DELETE("", h.revokeProjectInvite)
		rInvite.POST("/resend", h.resendProjectInvite)

		r.POST("/transfer-ownership", h.transferProjectOwnership)

		rUser := r.Group("/users/{userId}")
		rUser.Use(httpUtils.ValidateAndSetId("userId"))
		rUser.DELETE("", h.removeMemberFromProject)
		rUser.PUT("", h.setMemberPrivilegeLevelInProject)
	}

	projectJWTDocRouter := projectJWTRouter.Group("/doc/{docId}")
	projectJWTDocRouter.Use(httpUtils.ValidateAndSetId("docId"))
	projectJWTDocRouter.POST("/metadata", h.getMetadataForDoc)
	return router
}

var err403 = &errors.NotAuthorizedError{}

func blockRestrictedUsers(next httpUtils.HandlerFunc) httpUtils.HandlerFunc {
	return func(c *httpUtils.Context) {
		if projectJWT.MustGet(c).IsRestrictedUser() {
			httpUtils.Respond(c, http.StatusOK, nil, err403)
			return
		}
		next(c)
	}
}

func requireProjectAdminAccess(next httpUtils.HandlerFunc) httpUtils.HandlerFunc {
	return func(c *httpUtils.Context) {
		err := projectJWT.MustGet(c).PrivilegeLevel.CheckIsAtLeast(
			sharedTypes.PrivilegeLevelOwner,
		)
		if err != nil {
			httpUtils.Respond(c, http.StatusOK, nil, err)
			return
		}
		next(c)
	}
}

func requireWriteAccess(next httpUtils.HandlerFunc) httpUtils.HandlerFunc {
	return func(c *httpUtils.Context) {
		err := projectJWT.MustGet(c).PrivilegeLevel.CheckIsAtLeast(
			sharedTypes.PrivilegeLevelReadAndWrite,
		)
		if err != nil {
			httpUtils.Respond(c, http.StatusOK, nil, err)
			return
		}
		next(c)
	}
}

func mustGetSignedCompileProjectOptionsFromJwt(c *httpUtils.Context) types.SignedCompileProjectRequestOptions {
	return projectJWT.MustGet(c).SignedCompileProjectRequestOptions
}

func (h *httpController) flushSession(c *httpUtils.Context, s *session.Session, err error) error {
	if err2 := h.wm.Flush(c, s); err == nil && err2 != nil {
		err = err2
	}
	return err
}

func (h *httpController) mustGetOrCreateSession(c *httpUtils.Context, request interface{ SetSession(s *session.Session) }, response interface{}) bool {
	s, err := h.wm.GetOrCreateSession(c)
	if err != nil {
		httpUtils.Respond(c, http.StatusOK, response, err)
		return false
	}
	request.SetSession(s)
	return true
}

func (h *httpController) mustGetOrCreateSessionHTML(c *httpUtils.Context, request interface{ SetSession(s *session.Session) }) bool {
	s, err := h.wm.GetOrCreateSession(c)
	if err != nil {
		templates.RespondHTML(c, nil, err, s, h.ps, h.wm.Flush)
		return false
	}
	request.SetSession(s)
	return true
}

func (h *httpController) mustRequireLoggedInSession(c *httpUtils.Context, request interface{ SetSession(s *session.Session) }) bool {
	s, err := h.wm.RequireLoggedInSession(c)
	if err != nil {
		httpUtils.RespondErr(c, err)
		return false
	}
	request.SetSession(s)
	return true
}

func (h *httpController) mustProcessQuery(request interface{ FromQuery(values url.Values) error }, c *httpUtils.Context) bool {
	if err := request.FromQuery(c.Request.URL.Query()); err != nil {
		httpUtils.RespondErr(c, err)
		return false
	}
	return true
}

func (h *httpController) mustProcessQueryHTML(request interface{ FromQuery(values url.Values) error }, c *httpUtils.Context, s *session.Session) bool {
	if err := request.FromQuery(c.Request.URL.Query()); err != nil {
		templates.RespondHTML(c, nil, err, s, h.ps, h.wm.Flush)
		return false
	}
	return true
}

func (h *httpController) mustProcessSignedOptions(request interface {
	FromSignedOptions(o types.SignedCompileProjectRequestOptions)
}, c *httpUtils.Context) {
	request.FromSignedOptions(mustGetSignedCompileProjectOptionsFromJwt(c))
}

func (h *httpController) addApiCSP(c *httpUtils.Context) {
	c.Writer.Header().Set("Content-Security-Policy", h.ps.CSPs.API)
}

func (h *httpController) addApiCSPmw(next httpUtils.HandlerFunc) httpUtils.HandlerFunc {
	return func(c *httpUtils.Context) {
		c.Writer.Header().Set("Content-Security-Policy", h.ps.CSPs.API)
		next(c)
	}
}

var unsupportedBrowsers = regexp.MustCompile("Trident/|MSIE")

func (h *httpController) blockUnsupportedBrowser(next httpUtils.HandlerFunc) httpUtils.HandlerFunc {
	return func(c *httpUtils.Context) {
		if unsupportedBrowsers.MatchString(c.Request.Header.Get("User-Agent")) {
			h.unsupportedBrowserPage(c)
			return
		}
		next(c)
	}
}

func (h *httpController) unsupportedBrowserPage(c *httpUtils.Context) {
	s, _ := h.wm.GetOrCreateSession(c)
	body := &templates.GeneralUnsupportedBrowserData{
		NoJsLayoutData: templates.NoJsLayoutData{
			CommonData: templates.CommonData{
				Settings:              h.ps,
				RobotsNoindexNofollow: true,
				Title:                 "Unsupported browser",
				Viewport:              true,
			},
		},
		FromURL: h.ps.SiteURL.
			WithPath(c.Request.URL.Path).
			WithQuery(c.Request.URL.Query()),
	}
	templates.RespondHTMLCustomStatus(
		c, http.StatusNotAcceptable, body, nil, s, h.ps, h.wm.Flush,
	)
}

func (h *httpController) notFound(c *httpUtils.Context) {
	err := &errors.NotFoundError{}
	p := c.Request.URL.Path
	if strings.HasPrefix(p, "/api") || strings.HasPrefix(p, "/jwt") {
		h.addApiCSP(c)
		httpUtils.RespondErr(c, err)
		return
	}
	s, _ := h.wm.GetOrCreateSession(c)
	templates.RespondHTML(c, nil, err, s, h.ps, h.wm.Flush)
}

func (h *httpController) switchLanguage(c *httpUtils.Context) {
	request := &types.SwitchLanguageRequest{}
	if !h.mustGetOrCreateSessionHTML(c, request) {
		return
	}
	if !h.mustProcessQueryHTML(request, c, request.Session) {
		return
	}
	request.Referrer = c.Request.Referer()
	response := &types.SwitchLanguageResponse{}
	if err := h.wm.SwitchLanguage(c, request, response); err != nil {
		templates.RespondHTML(c, nil, err, request.Session, h.ps, h.wm.Flush)
		return
	}
	httpUtils.Redirect(c, response.Redirect)
}

func (h *httpController) clearProjectCache(c *httpUtils.Context) {
	request := &types.ClearCompileCacheRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	o := mustGetSignedCompileProjectOptionsFromJwt(c)
	request.SignedCompileProjectRequestOptions = o
	err := h.wm.ClearCache(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) compileProject(c *httpUtils.Context) {
	request := &types.CompileProjectRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	o := mustGetSignedCompileProjectOptionsFromJwt(c)
	request.SignedCompileProjectRequestOptions = o
	response := &types.CompileProjectResponse{}
	err := h.wm.Compile(c, request, response)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) syncFromCode(c *httpUtils.Context) {
	request := &types.SyncFromCodeRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	o := mustGetSignedCompileProjectOptionsFromJwt(c)
	request.SignedCompileProjectRequestOptions = o

	response := &clsiTypes.PDFPositions{}
	err := h.wm.SyncFromCode(c, request, response)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) syncFromPDF(c *httpUtils.Context) {
	request := &types.SyncFromPDFRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	o := mustGetSignedCompileProjectOptionsFromJwt(c)
	request.SignedCompileProjectRequestOptions = o

	response := &clsiTypes.CodePositions{}
	err := h.wm.SyncFromPDF(c, request, response)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) wordCount(c *httpUtils.Context) {
	request := &types.WordCountRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	o := mustGetSignedCompileProjectOptionsFromJwt(c)
	request.SignedCompileProjectRequestOptions = o

	response := &clsiTypes.Words{}
	err := h.wm.WordCount(c, request, response)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) getSystemMessages(c *httpUtils.Context) {
	m, err := h.wm.GetAllCached(c, httpUtils.GetId(c, "userId"))
	httpUtils.Respond(c, http.StatusOK, m, err)
}

func (h *httpController) getUserProjects(c *httpUtils.Context) {
	request := &types.GetUserProjectsRequest{}
	response := &types.GetUserProjectsResponse{}
	if !h.mustGetOrCreateSession(c, request, response) {
		return
	}
	err := h.wm.GetUserProjects(c, request, response)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) getMetadataForProject(c *httpUtils.Context) {
	projectId := mustGetSignedCompileProjectOptionsFromJwt(c).ProjectId
	response, err := h.wm.GetMetadataForProject(c, projectId)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) getMetadataForDoc(c *httpUtils.Context) {
	request := &types.ProjectDocMetadataRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	projectId := mustGetSignedCompileProjectOptionsFromJwt(c).ProjectId
	docId := httpUtils.GetId(c, "docId")
	response, err := h.wm.GetMetadataForDoc(c, projectId, docId, request)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) login(c *httpUtils.Context) {
	request := &types.LoginRequest{}
	response := &types.LoginResponse{}
	if !h.mustGetOrCreateSession(c, request, response) {
		return
	}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.IPAddress = c.ClientIP()
	err := h.wm.Login(c, request, response)
	err = h.flushSession(c, request.Session, err)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) logout(c *httpUtils.Context) {
	request := &types.LogoutRequest{}
	response := &types.LogoutResponse{
		RedirectTo: "/login",
	}
	if !h.mustGetOrCreateSession(c, request, response) {
		return
	}
	err := h.wm.Logout(c, request)
	err = h.flushSession(c, request.Session, err)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) getLoggedInUserJWT(c *httpUtils.Context) {
	request := &types.GetLoggedInUserJWTRequest{}
	response := types.GetLoggedInUserJWTResponse("")
	if !h.mustGetOrCreateSession(c, request, response) {
		return
	}
	err := h.wm.GetLoggedInUserJWT(c, request, &response)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) getProjectJWT(c *httpUtils.Context) {
	request := &types.GetProjectJWTRequest{
		ProjectId: httpUtils.GetId(c, "projectId"),
	}
	response := types.GetProjectJWTResponse("")
	if !h.mustGetOrCreateSession(c, request, response) {
		return
	}
	err := h.wm.GetProjectJWT(c, request, &response)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) getWSBootstrap(c *httpUtils.Context) {
	request := &types.GetWSBootstrapRequest{
		ProjectId: httpUtils.GetId(c, "projectId"),
	}
	response := types.GetWSBootstrapResponse{}
	if !h.mustGetOrCreateSession(c, request, response) {
		return
	}
	err := h.wm.GetWSBootstrap(c, request, &response)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) getProjectMessages(c *httpUtils.Context) {
	request := &types.GetProjectChatMessagesRequest{}
	if !h.mustProcessQuery(request, c) {
		return
	}
	request.ProjectId = projectJWT.MustGet(c).ProjectId
	response := make(types.GetProjectChatMessagesResponse, 0)
	err := h.wm.GetProjectMessages(c, request, &response)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) sendProjectMessage(c *httpUtils.Context) {
	request := &types.SendProjectChatMessageRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.ProjectId = projectJWT.MustGet(c).ProjectId
	request.UserId = projectJWT.MustGet(c).UserId
	err := h.wm.SendProjectMessage(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) optInBetaProgram(c *httpUtils.Context) {
	request := &types.OptInBetaProgramRequest{}
	if !h.mustGetOrCreateSession(c, request, nil) {
		return
	}
	err := h.wm.OptInBetaProgram(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) optOutBetaProgram(c *httpUtils.Context) {
	request := &types.OptOutBetaProgramRequest{}
	if !h.mustGetOrCreateSession(c, request, nil) {
		return
	}
	err := h.wm.OptOutBetaProgram(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) getProjectEntities(c *httpUtils.Context) {
	request := &types.GetProjectEntitiesRequest{
		ProjectId: httpUtils.GetId(c, "projectId"),
	}
	response := &types.GetProjectEntitiesResponse{}
	if !h.mustGetOrCreateSession(c, request, response) {
		return
	}
	err := h.wm.GetProjectEntities(c, request, response)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) grantTokenAccessReadAndWrite(c *httpUtils.Context) {
	request := &types.GrantTokenAccessRequest{
		Token: project.AccessToken(c.Param("token")),
	}
	response := &types.GrantTokenAccessResponse{}
	if !h.mustGetOrCreateSession(c, request, response) {
		return
	}
	err := h.wm.GrantTokenAccessReadAndWrite(c, request, response)
	err = h.flushSession(c, request.Session, err)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) grantTokenAccessReadOnly(c *httpUtils.Context) {
	request := &types.GrantTokenAccessRequest{
		Token: project.AccessToken(c.Param("token")),
	}
	response := &types.GrantTokenAccessResponse{}
	if !h.mustGetOrCreateSession(c, request, response) {
		return
	}
	err := h.wm.GrantTokenAccessReadOnly(c, request, response)
	err = h.flushSession(c, request.Session, err)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) addProjectToTag(c *httpUtils.Context) {
	request := &types.AddProjectToTagRequest{
		ProjectId: httpUtils.GetId(c, "projectId"),
		TagId:     httpUtils.GetId(c, "tagId"),
	}
	if !h.mustGetOrCreateSession(c, request, nil) {
		return
	}
	err := h.wm.AddProjectToTag(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) createTag(c *httpUtils.Context) {
	request := &types.CreateTagRequest{}
	response := &types.CreateTagResponse{}
	if !h.mustGetOrCreateSession(c, request, response) {
		return
	}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	err := h.wm.CreateTag(c, request, response)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) deleteTag(c *httpUtils.Context) {
	request := &types.DeleteTagRequest{
		TagId: httpUtils.GetId(c, "tagId"),
	}
	if !h.mustGetOrCreateSession(c, request, nil) {
		return
	}
	err := h.wm.DeleteTag(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) renameTag(c *httpUtils.Context) {
	request := &types.RenameTagRequest{}
	if !h.mustGetOrCreateSession(c, request, nil) {
		return
	}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.TagId = httpUtils.GetId(c, "tagId")
	err := h.wm.RenameTag(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) removeProjectToTag(c *httpUtils.Context) {
	request := &types.RemoveProjectToTagRequest{
		ProjectId: httpUtils.GetId(c, "projectId"),
		TagId:     httpUtils.GetId(c, "tagId"),
	}
	if !h.mustGetOrCreateSession(c, request, nil) {
		return
	}
	err := h.wm.RemoveProjectFromTag(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) getUserContacts(c *httpUtils.Context) {
	request := &types.GetUserContactsRequest{}
	response := &types.GetUserContactsResponse{}
	if !h.mustGetOrCreateSession(c, request, response) {
		return
	}
	err := h.wm.GetUserContacts(c, request, response)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) archiveProject(c *httpUtils.Context) {
	request := &types.ArchiveProjectRequest{
		ProjectId: httpUtils.GetId(c, "projectId"),
	}
	if !h.mustGetOrCreateSession(c, request, nil) {
		return
	}
	err := h.wm.ArchiveProject(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) unArchiveProject(c *httpUtils.Context) {
	request := &types.UnArchiveProjectRequest{
		ProjectId: httpUtils.GetId(c, "projectId"),
	}
	if !h.mustGetOrCreateSession(c, request, nil) {
		return
	}
	err := h.wm.UnArchiveProject(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) trashProject(c *httpUtils.Context) {
	request := &types.TrashProjectRequest{
		ProjectId: httpUtils.GetId(c, "projectId"),
	}
	if !h.mustGetOrCreateSession(c, request, nil) {
		return
	}
	err := h.wm.TrashProject(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) unTrashProject(c *httpUtils.Context) {
	request := &types.UnTrashProjectRequest{
		ProjectId: httpUtils.GetId(c, "projectId"),
	}
	if !h.mustGetOrCreateSession(c, request, nil) {
		return
	}
	err := h.wm.UnTrashProject(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func prepareFileResponse(
	c *httpUtils.Context, filename sharedTypes.Filename, size int64,
) {
	httpUtils.EndTotalTimer(c)
	cd := fmt.Sprintf("attachment; filename=%q", filename)
	c.Writer.Header().Set("Content-Disposition", cd)
	c.Writer.Header().Set("Content-Type", "application/octet-stream")
	c.Writer.Header().Set(
		"Content-Length", strconv.FormatInt(size, 10),
	)
	c.Writer.WriteHeader(http.StatusOK)
}

func (h *httpController) getProjectFile(c *httpUtils.Context) {
	request := &types.GetProjectFileRequest{
		ProjectId: httpUtils.GetId(c, "projectId"),
		FileId:    httpUtils.GetId(c, "fileId"),
	}
	if !h.mustGetOrCreateSession(c, request, nil) {
		return
	}
	response := &types.GetProjectFileResponse{}
	if err := h.wm.GetProjectFile(c, request, response); err != nil {
		httpUtils.Respond(c, http.StatusOK, nil, err)
		return
	}
	prepareFileResponse(c, response.Filename, response.Size)
	_, _ = io.Copy(c.Writer, response.Reader)
	_ = response.Reader.Close()
}

func (h *httpController) getProjectFileSize(c *httpUtils.Context) {
	request := &types.GetProjectFileSizeRequest{
		ProjectId: httpUtils.GetId(c, "projectId"),
		FileId:    httpUtils.GetId(c, "fileId"),
	}
	if !h.mustGetOrCreateSession(c, request, nil) {
		return
	}
	response := &types.GetProjectFileSizeResponse{}
	if err := h.wm.GetProjectFileSize(c, request, response); err != nil {
		httpUtils.Respond(c, http.StatusOK, nil, err)
		return
	}
	prepareFileResponse(c, response.Filename, response.Size)
}

func (h *httpController) addDocToProject(c *httpUtils.Context) {
	request := &types.AddDocRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	h.mustProcessSignedOptions(request, c)
	response := &types.AddDocResponse{}
	err := h.wm.AddDocToProject(c, request, response)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) addFolderToProject(c *httpUtils.Context) {
	request := &types.AddFolderRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	h.mustProcessSignedOptions(request, c)
	response := &types.AddFolderResponse{}
	err := h.wm.AddFolderToProject(c, request, response)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) uploadFile(c *httpUtils.Context) {
	j := projectJWT.MustGet(c)
	d := &httpUtils.UploadDetails{}
	if !httpUtils.ProcessFileUpload(d, c, types.MaxUploadSize, maxDocSize) {
		return
	}
	defer d.Cleanup()
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
	err := h.wm.UploadFile(c, request)
	httpUtils.Respond(c, http.StatusOK, asyncForm.Response{}, err)
}

func (h *httpController) deleteDocFromProject(c *httpUtils.Context) {
	request := &types.DeleteDocRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	h.mustProcessSignedOptions(request, c)
	request.DocId = httpUtils.GetId(c, "docId")
	err := h.wm.DeleteDocFromProject(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) deleteFileFromProject(c *httpUtils.Context) {
	request := &types.DeleteFileRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	h.mustProcessSignedOptions(request, c)
	request.FileId = httpUtils.GetId(c, "fileId")
	err := h.wm.DeleteFileFromProject(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) deleteFolderFromProject(c *httpUtils.Context) {
	request := &types.DeleteFolderRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	h.mustProcessSignedOptions(request, c)
	request.FolderId = httpUtils.GetId(c, "folderId")
	err := h.wm.DeleteFolderFromProject(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) moveDocInProject(c *httpUtils.Context) {
	request := &types.MoveDocRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	h.mustProcessSignedOptions(request, c)
	request.DocId = httpUtils.GetId(c, "docId")
	err := h.wm.MoveDocInProject(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) moveFileInProject(c *httpUtils.Context) {
	request := &types.MoveFileRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	h.mustProcessSignedOptions(request, c)
	request.FileId = httpUtils.GetId(c, "fileId")
	err := h.wm.MoveFileInProject(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) moveFolderInProject(c *httpUtils.Context) {
	request := &types.MoveFolderRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	h.mustProcessSignedOptions(request, c)
	request.FolderId = httpUtils.GetId(c, "folderId")
	err := h.wm.MoveFolderInProject(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) renameDocInProject(c *httpUtils.Context) {
	request := &types.RenameDocRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	h.mustProcessSignedOptions(request, c)
	request.DocId = httpUtils.GetId(c, "docId")
	err := h.wm.RenameDocInProject(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) renameFileInProject(c *httpUtils.Context) {
	request := &types.RenameFileRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	h.mustProcessSignedOptions(request, c)
	request.FileId = httpUtils.GetId(c, "fileId")
	err := h.wm.RenameFileInProject(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) renameFolderInProject(c *httpUtils.Context) {
	request := &types.RenameFolderRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	h.mustProcessSignedOptions(request, c)
	request.FolderId = httpUtils.GetId(c, "folderId")
	err := h.wm.RenameFolderInProject(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) restoreDeletedDocInProject(c *httpUtils.Context) {
	request := &types.RestoreDeletedDocRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	h.mustProcessSignedOptions(request, c)
	request.DocId = httpUtils.GetId(c, "docId")
	response := &types.RestoreDeletedDocResponse{}
	err := h.wm.RestoreDeletedDocInProject(c, request, response)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) renameProject(c *httpUtils.Context) {
	request := &types.RenameProjectRequest{}
	if !h.mustGetOrCreateSession(c, request, nil) {
		return
	}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.ProjectId = httpUtils.GetId(c, "projectId")
	err := h.wm.RenameProject(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) acceptProjectInvite(c *httpUtils.Context) {
	request := &types.AcceptProjectInviteRequest{
		ProjectId: httpUtils.GetId(c, "projectId"),
		Token:     projectInvite.Token(c.Param("token")),
	}
	response := &types.AcceptProjectInviteResponse{}
	if !h.mustGetOrCreateSession(c, request, response) {
		return
	}
	err := h.wm.AcceptProjectInvite(c, request, response)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) createProjectInvite(c *httpUtils.Context) {
	request := &types.CreateProjectInviteRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	h.mustProcessSignedOptions(request, c)
	err := h.wm.CreateProjectInvite(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) resendProjectInvite(c *httpUtils.Context) {
	request := &types.ResendProjectInviteRequest{
		InviteId: httpUtils.GetId(c, "inviteId"),
	}
	h.mustProcessSignedOptions(request, c)
	err := h.wm.ResendProjectInvite(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) revokeProjectInvite(c *httpUtils.Context) {
	request := &types.RevokeProjectInviteRequest{
		InviteId: httpUtils.GetId(c, "inviteId"),
	}
	h.mustProcessSignedOptions(request, c)
	err := h.wm.RevokeProjectInvite(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) listProjectInvites(c *httpUtils.Context) {
	request := &types.ListProjectInvitesRequest{}
	h.mustProcessSignedOptions(request, c)
	response := &types.ListProjectInvitesResponse{}
	err := h.wm.ListProjectInvites(c, request, response)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) listProjectMembers(c *httpUtils.Context) {
	request := &types.ListProjectMembersRequest{}
	h.mustProcessSignedOptions(request, c)
	response := &types.ListProjectMembersResponse{}
	err := h.wm.ListProjectMembers(c, request, response)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) removeMemberFromProject(c *httpUtils.Context) {
	request := &types.RemoveProjectMemberRequest{
		MemberId: httpUtils.GetId(c, "userId"),
	}
	h.mustProcessSignedOptions(request, c)
	err := h.wm.RemoveMemberFromProject(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) setMemberPrivilegeLevelInProject(c *httpUtils.Context) {
	request := &types.SetMemberPrivilegeLevelInProjectRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	h.mustProcessSignedOptions(request, c)
	request.MemberId = httpUtils.GetId(c, "userId")
	err := h.wm.SetMemberPrivilegeLevelInProject(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) transferProjectOwnership(c *httpUtils.Context) {
	request := &types.TransferProjectOwnershipRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	h.mustProcessSignedOptions(request, c)
	err := h.wm.TransferProjectOwnership(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) leaveProject(c *httpUtils.Context) {
	request := &types.LeaveProjectRequest{
		ProjectId: httpUtils.GetId(c, "projectId"),
	}
	if !h.mustGetOrCreateSession(c, request, nil) {
		return
	}
	err := h.wm.LeaveProject(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) setCompiler(c *httpUtils.Context) {
	request := &types.SetCompilerRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	h.mustProcessSignedOptions(request, c)
	err := h.wm.SetCompiler(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) setImageName(c *httpUtils.Context) {
	request := &types.SetImageNameRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	h.mustProcessSignedOptions(request, c)
	err := h.wm.SetImageName(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) setSpellCheckLanguage(c *httpUtils.Context) {
	request := &types.SetSpellCheckLanguageRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	h.mustProcessSignedOptions(request, c)
	err := h.wm.SetSpellCheckLanguage(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) setRootDocId(c *httpUtils.Context) {
	request := &types.SetRootDocIdRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	h.mustProcessSignedOptions(request, c)
	err := h.wm.SetRootDocId(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) setPublicAccessLevel(c *httpUtils.Context) {
	request := &types.SetPublicAccessLevelRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	h.mustProcessSignedOptions(request, c)
	request.Epoch = projectJWT.MustGet(c).Epoch
	err := h.wm.SetPublicAccessLevel(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) clearSessions(c *httpUtils.Context) {
	request := &types.ClearSessionsRequest{
		IPAddress: c.ClientIP(),
	}
	if !h.mustGetOrCreateSession(c, request, nil) {
		return
	}
	err := h.wm.ClearSessions(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) cloneProject(c *httpUtils.Context) {
	request := &types.CloneProjectRequest{}
	if !h.mustRequireLoggedInSession(c, request) {
		return
	}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.ProjectId = httpUtils.GetId(c, "projectId")
	response := &types.CloneProjectResponse{}
	err := h.wm.CloneProject(c, request, response)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) createExampleProject(c *httpUtils.Context) {
	request := &types.CreateExampleProjectRequest{}
	if !h.mustRequireLoggedInSession(c, request) {
		return
	}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	response := &types.CreateExampleProjectResponse{}
	err := h.wm.CreateExampleProject(c, request, response)
	if err != nil && errors.IsValidationError(err) {
		response.Error = "Error: " + err.Error()
	}
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) createFromZip(c *httpUtils.Context) {
	request := &types.CreateProjectFromZipRequest{}
	if !h.mustRequireLoggedInSession(c, request) {
		return
	}

	d := &httpUtils.UploadDetails{}
	if !httpUtils.ProcessFileUpload(d, c, types.MaxUploadSize, maxDocSize) {
		return
	}
	request.UploadDetails = types.UploadDetails{
		File:     d.File,
		FileName: d.FileName,
		Size:     d.Size,
	}
	defer d.Cleanup()
	response := &types.CreateProjectResponse{}
	err := h.wm.CreateFromZip(c, request, response)
	if err != nil && errors.IsValidationError(err) {
		response.Error = "Error: " + err.Error()
	}
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) getUserNotifications(c *httpUtils.Context) {
	request := &types.GetNotificationsRequest{}
	if !h.mustRequireLoggedInSession(c, request) {
		return
	}
	response := &types.GetNotificationsResponse{}
	err := h.wm.GetUserNotifications(c, request, response)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) removeNotification(c *httpUtils.Context) {
	request := &types.RemoveNotificationRequest{
		NotificationId: httpUtils.GetId(c, "notificationId"),
	}
	if !h.mustRequireLoggedInSession(c, request) {
		return
	}
	err := h.wm.RemoveNotification(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) openInOverleaf(c *httpUtils.Context) {
	request := &types.OpenInOverleafRequest{}
	if !h.mustRequireLoggedInSession(c, request) {
		return
	}
	if c.Request.Header.Get("Content-Type") == "application/json" {
		if !httpUtils.MustParseJSON(request, c) {
			return
		}
	} else {
		if err := c.Request.ParseMultipartForm(0); err != nil {
			httpUtils.RespondErr(c, &errors.ValidationError{Msg: err.Error()})
			return
		}
		if err := request.PopulateFromParams(c.Request.Form); err != nil {
			httpUtils.RespondErr(c, err)
			return
		}
	}
	response := &types.CreateProjectResponse{}
	err := h.wm.OpenInOverleaf(c, request, response)
	if err != nil && errors.IsValidationError(err) {
		response.Error = err.Error()
	}
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) compileProjectHeadless(c *httpUtils.Context) {
	request := &types.CompileProjectHeadlessRequest{
		ProjectId: httpUtils.GetId(c, "projectId"),
	}
	if !h.mustRequireLoggedInSession(c, request) {
		return
	}
	request.UserId = request.Session.User.Id
	response := &types.CompileProjectResponse{}
	err := h.wm.CompileHeadLess(c, request, response)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) createLinkedFile(c *httpUtils.Context) {
	request := &types.CreateLinkedFileRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	h.mustProcessSignedOptions(request, c)
	err := h.wm.CreateLinkedFile(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) refreshLinkedFile(c *httpUtils.Context) {
	request := &types.RefreshLinkedFileRequest{
		FileId: httpUtils.GetId(c, "fileId"),
	}
	h.mustProcessSignedOptions(request, c)
	err := h.wm.RefreshLinkedFile(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) createProjectZIP(c *httpUtils.Context) {
	request := &types.CreateProjectZIPRequest{
		ProjectId: httpUtils.GetId(c, "projectId"),
	}
	if !h.mustGetOrCreateSession(c, request, nil) {
		return
	}
	response := &types.CreateProjectZIPResponse{}
	defer response.Cleanup()

	if err := h.wm.CreateProjectZIP(c, request, response); err != nil {
		httpUtils.RespondErr(c, err)
		return
	}
	cd := fmt.Sprintf("attachment; filename=%q", response.Filename)
	c.Writer.Header().Set("Content-Disposition", cd)
	httpUtils.EndTotalTimer(c)
	http.ServeFile(c.Writer, c.Request, response.FSPath)
}

func (h *httpController) createMultiProjectZIP(c *httpUtils.Context) {
	request := &types.CreateMultiProjectZIPRequest{}
	if !h.mustRequireLoggedInSession(c, request) {
		return
	}
	if !h.mustProcessQuery(request, c) {
		return
	}
	response := &types.CreateProjectZIPResponse{}
	defer response.Cleanup()

	if err := h.wm.CreateMultiProjectZIP(c, request, response); err != nil {
		httpUtils.RespondErr(c, err)
		return
	}
	cd := fmt.Sprintf("attachment; filename=%q", response.Filename)
	c.Writer.Header().Set("Content-Disposition", cd)
	httpUtils.EndTotalTimer(c)
	http.ServeFile(c.Writer, c.Request, response.FSPath)
}

func (h *httpController) deleteProject(c *httpUtils.Context) {
	request := &types.DeleteProjectRequest{
		ProjectId: httpUtils.GetId(c, "projectId"),
		IPAddress: c.ClientIP(),
	}
	if !h.mustRequireLoggedInSession(c, request) {
		return
	}
	err := h.wm.DeleteProject(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) undeleteProject(c *httpUtils.Context) {
	request := &types.UnDeleteProjectRequest{
		ProjectId: httpUtils.GetId(c, "projectId"),
	}
	if !h.mustRequireLoggedInSession(c, request) {
		return
	}
	err := h.wm.UnDeleteProject(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) deleteUser(c *httpUtils.Context) {
	request := &types.DeleteUserRequest{}
	if !h.mustRequireLoggedInSession(c, request) {
		return
	}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.IPAddress = c.ClientIP()
	err := h.wm.DeleteUser(c, request)
	_ = h.wm.Flush(c, request.Session)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) updateEditorConfig(c *httpUtils.Context) {
	request := &types.UpdateEditorConfigRequest{}
	if !h.mustRequireLoggedInSession(c, request) {
		return
	}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	err := h.wm.UpdateEditorConfig(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) changeEmailAddress(c *httpUtils.Context) {
	request := &types.ChangeEmailAddressRequest{}
	if !h.mustRequireLoggedInSession(c, request) {
		return
	}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.IPAddress = c.ClientIP()
	err := h.wm.ChangeEmailAddress(c, request)
	_ = h.wm.Flush(c, request.Session)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) setUserName(c *httpUtils.Context) {
	request := &types.SetUserName{}
	if !h.mustRequireLoggedInSession(c, request) {
		return
	}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	err := h.wm.SetUserName(c, request)
	_ = h.wm.Flush(c, request.Session)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) changePassword(c *httpUtils.Context) {
	request := &types.ChangePasswordRequest{}
	if !h.mustRequireLoggedInSession(c, request) {
		return
	}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.IPAddress = c.ClientIP()
	res := &types.ChangePasswordResponse{}
	err := h.wm.ChangePassword(c, request, res)
	httpUtils.Respond(c, http.StatusOK, res, err)
}

func (h *httpController) requestPasswordReset(c *httpUtils.Context) {
	request := &types.RequestPasswordResetRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	err := h.wm.RequestPasswordReset(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) setPassword(c *httpUtils.Context) {
	request := &types.SetPasswordRequest{}
	if !h.mustGetOrCreateSession(c, request, nil) {
		return
	}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.IPAddress = c.ClientIP()
	res := &types.SetPasswordResponse{}
	err := h.wm.SetPassword(c, request, res)
	httpUtils.Respond(c, http.StatusOK, res, err)
}

func (h *httpController) confirmEmail(c *httpUtils.Context) {
	request := &types.ConfirmEmailRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	err := h.wm.ConfirmEmail(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) resendEmailConfirmation(c *httpUtils.Context) {
	request := &types.ResendEmailConfirmationRequest{}
	if !h.mustRequireLoggedInSession(c, request) {
		return
	}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	err := h.wm.ResendEmailConfirmation(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) getProjectHistoryUpdates(c *httpUtils.Context) {
	request := &types.GetProjectHistoryUpdatesRequest{}
	if !h.mustProcessQuery(request, c) {
		return
	}
	o := mustGetSignedCompileProjectOptionsFromJwt(c)
	request.ProjectId = o.ProjectId
	request.UserId = o.UserId
	res := &types.GetProjectHistoryUpdatesResponse{}
	err := h.wm.GetProjectHistoryUpdates(c, request, res)
	httpUtils.Respond(c, http.StatusOK, res, err)
}

func (h *httpController) getProjectDocDiff(c *httpUtils.Context) {
	request := &types.GetDocDiffRequest{}
	if !h.mustProcessQuery(request, c) {
		return
	}
	o := mustGetSignedCompileProjectOptionsFromJwt(c)
	request.ProjectId = o.ProjectId
	request.DocId = httpUtils.GetId(c, "docId")
	request.UserId = o.UserId
	res := &types.GetDocDiffResponse{}
	err := h.wm.GetDocDiff(c, request, res)
	httpUtils.Respond(c, http.StatusOK, res, err)
}

func (h *httpController) restoreDocVersion(c *httpUtils.Context) {
	i, err := strconv.ParseInt(c.Param("version"), 10, 64)
	if err != nil {
		httpUtils.RespondErr(c, &errors.ValidationError{Msg: err.Error()})
		return
	}
	o := mustGetSignedCompileProjectOptionsFromJwt(c)
	request := &types.RestoreDocVersionRequest{
		ProjectId: o.ProjectId,
		DocId:     httpUtils.GetId(c, "docId"),
		UserId:    o.UserId,
		FromV:     sharedTypes.Version(i),
	}
	err = h.wm.RestoreDocVersion(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) registerUser(c *httpUtils.Context) {
	response := &types.RegisterUserResponse{}
	request := &types.RegisterUserRequest{}
	if !h.mustGetOrCreateSession(c, request, response) {
		return
	}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.IPAddress = c.ClientIP()
	err := h.wm.RegisterUser(c, request, response)
	if err2 := h.wm.Flush(c, request.Session); err == nil && err2 != nil {
		response.RedirectTo = "/login"
	}
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) adminCreateUser(c *httpUtils.Context) {
	request := &types.AdminCreateUserRequest{}
	if !h.mustRequireLoggedInSession(c, request) {
		return
	}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	request.IPAddress = c.ClientIP()
	response := &types.AdminCreateUserResponse{}
	err := h.wm.AdminCreateUser(c, request, response)
	httpUtils.Respond(c, http.StatusOK, response, err)
}

func (h *httpController) homePage(c *httpUtils.Context) {
	request := &types.HomepageRequest{}
	if !h.mustGetOrCreateSessionHTML(c, request) {
		return
	}
	if request.Session.IsLoggedIn() {
		httpUtils.Redirect(c, "/project")
	} else {
		httpUtils.Redirect(c, "/login")
	}
}

func (h *httpController) betaProgramParticipatePage(c *httpUtils.Context) {
	request := &types.BetaProgramParticipatePageRequest{}
	if !h.mustGetOrCreateSessionHTML(c, request) {
		return
	}
	res := &types.BetaProgramParticipatePageResponse{}
	err := h.wm.BetaProgramParticipatePage(c, request, res)
	templates.RespondHTML(c, res.Data, err, request.Session, h.ps, h.wm.Flush)
}

func (h *httpController) loginPage(c *httpUtils.Context) {
	request := &types.LoginPageRequest{
		Referrer: c.Request.Referer(),
	}
	if !h.mustGetOrCreateSessionHTML(c, request) {
		return
	}
	res := &types.LoginPageResponse{}
	err := h.wm.LoginPage(c, request, res)
	if err == nil && res.Redirect != "" {
		httpUtils.Redirect(c, res.Redirect)
		return
	}
	if err == nil {
		err = h.wm.Flush(c, request.Session)
	}
	templates.RespondHTML(c, res.Data, err, request.Session, h.ps, h.wm.Flush)
}

func (h *httpController) logoutPage(c *httpUtils.Context) {
	request := &types.LogoutPageRequest{}
	if !h.mustGetOrCreateSessionHTML(c, request) {
		return
	}
	res := &types.LogoutPageResponse{}
	err := h.wm.LogoutPage(c, request, res)
	templates.RespondHTML(c, res.Data, err, request.Session, h.ps, h.wm.Flush)
}

func (h *httpController) confirmEmailPage(c *httpUtils.Context) {
	request := &types.ConfirmEmailPageRequest{}
	if !h.mustGetOrCreateSessionHTML(c, request) {
		return
	}
	if !h.mustProcessQueryHTML(request, c, request.Session) {
		return
	}
	res := &types.ConfirmEmailPageResponse{}
	err := h.wm.ConfirmEmailPage(c, request, res)
	templates.RespondHTML(c, res.Data, err, request.Session, h.ps, h.wm.Flush)
}

func (h *httpController) reconfirmAccountPage(c *httpUtils.Context) {
	request := &types.ReconfirmAccountPageRequest{}
	if !h.mustGetOrCreateSessionHTML(c, request) {
		return
	}
	res := &types.ReconfirmAccountPageResponse{}
	err := h.wm.ReconfirmAccountPage(c, request, res)
	if err == nil && res.Redirect != "" {
		httpUtils.Redirect(c, res.Redirect)
		return
	}
	templates.RespondHTML(c, res.Data, err, request.Session, h.ps, h.wm.Flush)
}

func (h *httpController) registerUserPage(c *httpUtils.Context) {
	request := &types.RegisterUserPageRequest{}
	if !h.mustGetOrCreateSessionHTML(c, request) {
		return
	}
	if !h.mustProcessQueryHTML(request, c, request.Session) {
		return
	}
	res := &types.RegisterUserPageResponse{}
	err := h.wm.RegisterUserPage(c, request, res)
	if err == nil && res.Redirect != "" {
		httpUtils.Redirect(c, res.Redirect)
		return
	}
	templates.RespondHTML(c, res.Data, err, request.Session, h.ps, h.wm.Flush)
}

func (h *httpController) restrictedPage(c *httpUtils.Context) {
	request := &types.RestrictedPageRequest{}
	if !h.mustGetOrCreateSessionHTML(c, request) {
		return
	}
	err := &errors.NotAuthorizedError{}
	templates.RespondHTML(c, nil, err, request.Session, h.ps, h.wm.Flush)
}

func (h *httpController) setPasswordPage(c *httpUtils.Context) {
	request := &types.SetPasswordPageRequest{}
	if !h.mustGetOrCreateSessionHTML(c, request) {
		return
	}
	if !h.mustProcessQueryHTML(request, c, request.Session) {
		return
	}
	res := &types.SetPasswordPageResponse{}
	err := h.wm.SetPasswordPage(c, request, res)
	if err == nil && res.Redirect != "" {
		if err = h.wm.Flush(c, request.Session); err == nil {
			httpUtils.Redirect(c, res.Redirect)
			return
		}
	}
	templates.RespondHTML(c, res.Data, err, request.Session, h.ps, h.wm.Flush)
}

func (h *httpController) requestPasswordResetPage(c *httpUtils.Context) {
	request := &types.RequestPasswordResetPageRequest{}
	if !h.mustGetOrCreateSessionHTML(c, request) {
		return
	}
	if !h.mustProcessQueryHTML(request, c, request.Session) {
		return
	}
	res := &types.RequestPasswordResetPageResponse{}
	err := h.wm.RequestPasswordResetPage(c, request, res)
	if err == nil && res.Redirect != "" {
		httpUtils.Redirect(c, res.Redirect)
		return
	}
	templates.RespondHTML(c, res.Data, err, request.Session, h.ps, h.wm.Flush)
}

func (h *httpController) activateUserPage(c *httpUtils.Context) {
	request := &types.ActivateUserPageRequest{}
	if !h.mustGetOrCreateSessionHTML(c, request) {
		return
	}
	if !h.mustProcessQueryHTML(request, c, request.Session) {
		return
	}
	res := &types.ActivateUserPageResponse{}
	err := h.wm.ActivateUserPage(c, request, res)
	if err == nil && res.Redirect != "" {
		httpUtils.Redirect(c, res.Redirect)
		return
	}
	templates.RespondHTML(c, res.Data, err, request.Session, h.ps, h.wm.Flush)
}

func (h *httpController) sessionsPage(c *httpUtils.Context) {
	request := &types.SessionsPageRequest{}
	if !h.mustGetOrCreateSessionHTML(c, request) {
		return
	}
	res := &types.SessionsPageResponse{}
	err := h.wm.SessionsPage(c, request, res)
	templates.RespondHTML(c, res.Data, err, request.Session, h.ps, h.wm.Flush)
}

func (h *httpController) settingsPage(c *httpUtils.Context) {
	request := &types.SettingsPageRequest{}
	if !h.mustGetOrCreateSessionHTML(c, request) {
		return
	}
	res := &types.SettingsPageResponse{}
	err := h.wm.SettingsPage(c, request, res)
	h.wm.TouchSession(c, request.Session)
	templates.RespondHTML(c, res.Data, err, request.Session, h.ps, h.wm.Flush)
}

func (h *httpController) tokenAccessPage(c *httpUtils.Context) {
	request := &types.TokenAccessPageRequest{
		Token: project.AccessToken(c.Param("token")),
	}
	if !h.mustGetOrCreateSessionHTML(c, request) {
		return
	}
	res := &types.TokenAccessPageResponse{}
	err := h.wm.TokenAccessPage(c, request, res)
	templates.RespondHTML(c, res.Data, err, request.Session, h.ps, h.wm.Flush)
}

func (h *httpController) openInOverleafDocumentationPage(c *httpUtils.Context) {
	request := &types.OpenInOverleafDocumentationPageRequest{}
	if !h.mustGetOrCreateSessionHTML(c, request) {
		return
	}
	res := &types.OpenInOverleafDocumentationPageResponse{}
	err := h.wm.OpenInOverleafDocumentationPage(c, request, res)
	templates.RespondHTML(c, res.Data, err, request.Session, h.ps, h.wm.Flush)
}

func (h *httpController) openInOverleafGatewayPage(c *httpUtils.Context) {
	request := &types.OpenInOverleafGatewayPageRequest{}
	if !h.mustGetOrCreateSessionHTML(c, request) {
		return
	}
	switch c.Request.Method {
	case http.MethodGet:
		request.Query = c.Request.URL.Query()
	case http.MethodPost:
		c.Request.Body = http.MaxBytesReader(
			c.Writer, c.Request.Body, types.MaxUploadSize,
		)
		var err error
		if c.Request.Header.Get("Content-Type") == "application/json" {
			var body []byte
			if body, err = io.ReadAll(c.Request.Body); err == nil {
				request.Body = body
			}
		} else {
			if err = c.Request.ParseForm(); err == nil {
				request.Query = c.Request.Form
			}
		}
		if err != nil {
			err = &errors.UnprocessableEntityError{
				Msg: "cannot read POST body",
			}
			templates.RespondHTML(c, nil, err, request.Session, h.ps, h.wm.Flush)
			return
		}
	default:
		err := &errors.ValidationError{Msg: "GET / POST allowed only"}
		templates.RespondHTML(c, nil, err, request.Session, h.ps, h.wm.Flush)
		return
	}
	res := &types.OpenInOverleafGatewayPageResponse{}
	err := h.wm.OpenInOverleafGatewayPage(c, request, res)
	templates.RespondHTML(c, res.Data, err, request.Session, h.ps, h.wm.Flush)
}

func (h *httpController) projectListPage(c *httpUtils.Context) {
	request := &types.ProjectListPageRequest{}
	if !h.mustGetOrCreateSessionHTML(c, request) {
		return
	}
	res := &types.ProjectListPageResponse{}
	err := h.wm.ProjectListPage(c, request, res)
	h.wm.TouchSession(c, request.Session)
	templates.RespondHTML(c, res.Data, err, request.Session, h.ps, h.wm.Flush)
}

func (h *httpController) projectEditorPage(c *httpUtils.Context) {
	request := &types.ProjectEditorPageRequest{}
	if !h.mustGetOrCreateSessionHTML(c, request) {
		return
	}
	projectId, err := httpUtils.ParseAndValidateId(c, "projectId")
	if err != nil {
		templates.RespondHTML(c, nil, err, request.Session, h.ps, h.wm.Flush)
		return
	}
	request.ProjectId = projectId
	res := &types.ProjectEditorPageResponse{}
	err = h.wm.ProjectEditorPage(c, request, res)
	h.wm.TouchSession(c, request.Session)
	templates.RespondHTML(c, res.Data, err, request.Session, h.ps, h.wm.Flush)
}

func (h *httpController) viewProjectInvitePage(c *httpUtils.Context) {
	request := &types.ViewProjectInvitePageRequest{}
	if !h.mustGetOrCreateSessionHTML(c, request) {
		return
	}
	projectId, err := httpUtils.ParseAndValidateId(c, "projectId")
	if err != nil {
		templates.RespondHTML(c, nil, err, request.Session, h.ps, h.wm.Flush)
		return
	}
	if !h.mustProcessQueryHTML(request, c, request.Session) {
		return
	}
	request.ProjectId = projectId
	request.Token = projectInvite.Token(c.Param("token"))
	res := &types.ViewProjectInvitePageResponse{}
	err = h.wm.ViewProjectInvite(c, request, res)
	if err == nil && res.Redirect != "" {
		httpUtils.Redirect(c, res.Redirect)
		return
	}
	templates.RespondHTML(c, res.Data, err, request.Session, h.ps, h.wm.Flush)
}

func (h *httpController) learn(c *httpUtils.Context) {
	request := &types.LearnPageRequest{
		Section:         c.Param("section"),
		Page:            c.Param("page"),
		HasQuestionMark: strings.HasSuffix(c.Request.RequestURI, "?"),
	}
	if t := request.PreSessionRedirect(c.Request.URL.EscapedPath()); t != "" {
		httpUtils.Redirect(c, t)
		return
	}
	if !h.mustGetOrCreateSessionHTML(c, request) {
		return
	}
	res := &types.LearnPageResponse{}
	err := h.wm.LearnPage(c, request, res)
	httpUtils.Age(c, res.Age)
	if err == nil && res.Redirect != "" {
		httpUtils.Redirect(c, res.Redirect)
		return
	}
	h.wm.TouchSession(c, request.Session)
	templates.RespondHTML(c, res.Data, err, request.Session, h.ps, h.wm.Flush)
}

func (h *httpController) proxyLearnImage(c *httpUtils.Context) {
	request := &types.LearnImageRequest{
		Path: sharedTypes.PathName(c.Request.URL.Path)[1:],
	}
	res := &types.LearnImageResponse{}
	if err := h.wm.ProxyImage(c, request, res); err != nil {
		httpUtils.RespondErr(c, err)
		return
	}
	c.Writer.Header().Set("Cache-Control", "public, max-age=604800")
	httpUtils.Age(c, res.Age)
	httpUtils.EndTotalTimer(c)
	http.ServeFile(c.Writer, c.Request, res.FSPath)
}

func (h *httpController) adminManageSitePage(c *httpUtils.Context) {
	request := &types.AdminManageSitePageRequest{}
	if !h.mustGetOrCreateSessionHTML(c, request) {
		return
	}
	res := &types.AdminManageSitePageResponse{}
	err := h.wm.AdminManageSitePage(c, request, res)
	templates.RespondHTML(c, res.Data, err, request.Session, h.ps, h.wm.Flush)
}

func (h *httpController) adminRegisterUsersPage(c *httpUtils.Context) {
	request := &types.AdminRegisterUsersPageRequest{}
	if !h.mustGetOrCreateSessionHTML(c, request) {
		return
	}
	res := &types.AdminRegisterUsersPageResponse{}
	err := h.wm.AdminRegisterUsersPage(c, request, res)
	templates.RespondHTML(c, res.Data, err, request.Session, h.ps, h.wm.Flush)
}

func (h *httpController) smokeTestAPI(c *httpUtils.Context) {
	err := h.wm.SmokeTestAPI(c)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) smokeTestFull(c *httpUtils.Context) {
	res := &types.SmokeTestResponse{}
	err := h.wm.SmokeTestFull(c, res)
	status := http.StatusOK
	if err != nil {
		status = http.StatusInternalServerError
		httpUtils.GetAndLogErrResponseDetails(c, err)
	}
	httpUtils.RespondWithIndent(c, status, res, nil)
}

func (h *httpController) getDictionary(c *httpUtils.Context) {
	request := &types.GetDictionaryRequest{}
	if !h.mustRequireLoggedInSession(c, request) {
		return
	}
	res := &types.GetDictionaryResponse{}
	err := h.wm.GetDictionary(c, request, res)
	httpUtils.Respond(c, http.StatusOK, res, err)
}

func (h *httpController) learnWord(c *httpUtils.Context) {
	request := &types.LearnWordRequest{}
	if !h.mustRequireLoggedInSession(c, request) {
		return
	}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	err := h.wm.LearnWord(c, request)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}
