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

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/httpUtils"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"

	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
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
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/status", h.status)
	router.HEAD("/status", h.status)

	projectRouter := router.Group("/project/:projectId")
	projectRouter.Use(httpUtils.ValidateAndSetId("projectId"))

	projectRouter.DELETE("", h.flushAndDeleteProject)
	projectRouter.POST("", h.processProjectUpdates)
	projectRouter.POST("/clearState", h.clearProjectState)
	projectRouter.POST("/flush", h.flushProject)
	projectRouter.POST("/get_and_flush_if_old", h.getAndFlushIfOld)

	docRouter := projectRouter.Group("/doc/:docId")
	docRouter.Use(httpUtils.ValidateAndSetId("docId"))

	docRouter.GET("", h.getDoc)
	docRouter.POST("", h.setDoc)
	docRouter.DELETE("", h.flushAndDeleteDoc)
	docRouter.POST("/flush", h.flushDocIfLoaded)
	docRouter.GET("/exists", h.checkDocExists)
	docRouter.HEAD("/exists", h.checkDocExists)
	return router
}

func (h *httpController) status(c *gin.Context) {
	c.String(http.StatusOK, "document-updater is alive (go)\n")
}

func (h *httpController) handle404(c *gin.Context) {
	httpUtils.Respond(c, http.StatusNotFound, nil, errors.New("404"))
}

func (h *httpController) checkDocExists(c *gin.Context) {
	err := h.dum.CheckDocExists(
		c.Request.Context(),
		httpUtils.GetId(c, "projectId"),
		httpUtils.GetId(c, "docId"),
	)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

type getDocRequestOptions struct {
	FromVersion sharedTypes.Version `form:"fromVersion" binding:"required"`
}

func (h *httpController) getDoc(c *gin.Context) {
	requestOptions := &getDocRequestOptions{}
	if err := c.ShouldBindQuery(requestOptions); err != nil {
		httpUtils.RespondErr(c, &errors.ValidationError{
			Msg: "invalid fromVersion",
		})
		return
	}
	doc, err := h.dum.GetDoc(
		c.Request.Context(),
		httpUtils.GetId(c, "projectId"),
		httpUtils.GetId(c, "docId"),
		requestOptions.FromVersion,
	)
	httpUtils.Respond(c, http.StatusOK, doc, err)
}

const (
	maxSetDocRequestSize = 8 * 1024 * 1024
)

func (h *httpController) setDoc(c *gin.Context) {
	n := c.Request.ContentLength
	if n > maxSetDocRequestSize {
		httpUtils.RespondErr(c, &errors.BodyTooLargeError{})
		return
	}
	request := &types.SetDocRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	if err := request.Validate(); err != nil {
		if err == sharedTypes.ErrDocIsTooLarge {
			c.JSON(http.StatusNotAcceptable, gin.H{"message": err.Error()})
			return
		}
		httpUtils.RespondErr(c, err)
		return
	}
	err := h.dum.SetDoc(
		c.Request.Context(),
		httpUtils.GetId(c, "projectId"),
		httpUtils.GetId(c, "docId"),
		request,
	)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) flushProject(c *gin.Context) {
	err := h.dum.FlushProject(
		c.Request.Context(),
		httpUtils.GetId(c, "projectId"),
	)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) flushDocIfLoaded(c *gin.Context) {
	err := h.dum.FlushDocIfLoaded(
		c.Request.Context(),
		httpUtils.GetId(c, "projectId"),
		httpUtils.GetId(c, "docId"),
	)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) flushAndDeleteDoc(c *gin.Context) {
	err := h.dum.FlushAndDeleteDoc(
		c.Request.Context(),
		httpUtils.GetId(c, "projectId"),
		httpUtils.GetId(c, "docId"),
	)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) flushAndDeleteProject(c *gin.Context) {
	err := h.dum.FlushAndDeleteProject(
		c.Request.Context(),
		httpUtils.GetId(c, "projectId"),
	)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) getAndFlushIfOld(c *gin.Context) {
	var docs interface{}
	var err error
	if c.Query("snapshot") == "true" {
		docs, err = h.dum.GetProjectDocsAndFlushIfOldSnapshot(
			c.Request.Context(),
			httpUtils.GetId(c, "projectId"),
		)
	} else {
		docs, err = h.dum.GetProjectDocsAndFlushIfOldLines(
			c.Request.Context(),
			httpUtils.GetId(c, "projectId"),
		)
	}
	httpUtils.Respond(c, http.StatusOK, docs, err)
}

func (h *httpController) clearProjectState(c *gin.Context) {
	httpUtils.Respond(c, http.StatusNoContent, nil, nil)
}

func (h *httpController) processProjectUpdates(c *gin.Context) {
	request := &types.ProcessProjectUpdatesRequest{}
	if !httpUtils.MustParseJSON(request, c) {
		return
	}
	err := h.dum.ProcessProjectUpdates(
		c.Request.Context(),
		httpUtils.GetId(c, "projectId"),
		request,
	)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}
