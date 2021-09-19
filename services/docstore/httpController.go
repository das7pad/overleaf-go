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
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/httpUtils"
	"github.com/das7pad/overleaf-go/pkg/models/doc"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/docstore/pkg/managers/docstore"
	"github.com/das7pad/overleaf-go/services/docstore/pkg/types"
)

func newHttpController(dm docstore.Manager) httpController {
	return httpController{dm: dm}
}

type httpController struct {
	dm docstore.Manager
}

func (h *httpController) GetRouter() http.Handler {
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/status", h.status)

	projectRouter := router.Group("/project/:projectId")
	projectRouter.Use(httpUtils.ValidateAndSetId("projectId"))

	docRouter := projectRouter.Group("/doc/:docId")
	docRouter.Use(httpUtils.ValidateAndSetId("docId"))

	projectRouter.GET("/doc-deleted", h.peakDeletedDocNames)
	projectRouter.GET("/doc", h.getAllDocContents)
	projectRouter.GET("/ranges", h.getAllRanges)
	projectRouter.POST("/archive", h.archiveProject)
	//goland:noinspection SpellCheckingInspection
	projectRouter.POST("/unarchive", h.unArchiveProject)
	projectRouter.POST("/destroy", h.destroyProject)

	docRouter.GET("", h.getDoc)
	docRouter.GET("/raw", h.getDocRaw)
	docRouter.GET("/deleted", h.isDocDeleted)
	docRouter.POST("", h.updateDoc)
	docRouter.PATCH("", h.patchDoc)
	docRouter.POST("/archive", h.archiveDoc)

	return router
}

func (h *httpController) status(c *gin.Context) {
	c.String(http.StatusOK, "docstore is alive (go)\n")
}

type peakDeletedDocNamesOptions struct {
	Limit types.Limit `form:"limit"`
}

func (h *httpController) peakDeletedDocNames(c *gin.Context) {
	requestOptions := &peakDeletedDocNamesOptions{}
	_ = c.ShouldBindQuery(requestOptions)
	limit := docstore.DefaultLimit
	if requestOptions.Limit > 0 {
		limit = requestOptions.Limit
	}

	docNames, err := h.dm.PeakDeletedDocNames(
		c,
		httpUtils.GetId(c, "projectId"),
		limit,
	)
	httpUtils.Respond(c, http.StatusOK, docNames, err)
}

func (h *httpController) getAllDocContents(c *gin.Context) {
	docs, err := h.dm.GetAllDocContents(
		c,
		httpUtils.GetId(c, "projectId"),
	)
	httpUtils.Respond(c, http.StatusOK, docs, err)
}

func (h *httpController) getAllRanges(c *gin.Context) {
	docNames, err := h.dm.GetAllRanges(
		c,
		httpUtils.GetId(c, "projectId"),
	)
	httpUtils.Respond(c, http.StatusOK, docNames, err)
}

func (h *httpController) archiveProject(c *gin.Context) {
	err := h.dm.ArchiveProject(
		c,
		httpUtils.GetId(c, "projectId"),
	)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) unArchiveProject(c *gin.Context) {
	err := h.dm.UnArchiveProject(
		c,
		httpUtils.GetId(c, "projectId"),
	)
	httpUtils.Respond(c, http.StatusOK, nil, err)
}

func (h *httpController) destroyProject(c *gin.Context) {
	err := h.dm.DestroyProject(
		c,
		httpUtils.GetId(c, "projectId"),
	)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) getDoc(c *gin.Context) {
	d, err := h.dm.GetFullDoc(
		c,
		httpUtils.GetId(c, "projectId"),
		httpUtils.GetId(c, "docId"),
	)
	if err == nil {
		if d.Deleted && c.Query("include_deleted") != "true" {
			c.Status(http.StatusNotFound)
			return
		}
	}
	httpUtils.Respond(c, http.StatusOK, d, err)
}

func (h *httpController) getDocRaw(c *gin.Context) {
	lines, err := h.dm.GetDocLines(
		c,
		httpUtils.GetId(c, "projectId"),
		httpUtils.GetId(c, "docId"),
	)
	if err != nil {
		httpUtils.Respond(c, http.StatusOK, lines, err)
		return
	}
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, strings.Join(lines, "\n"))
}

type isDocDeletedResponseBody struct {
	Deleted bool `json:"deleted"`
}

func (h *httpController) isDocDeleted(c *gin.Context) {
	deleted, err := h.dm.IsDocDeleted(
		c,
		httpUtils.GetId(c, "projectId"),
		httpUtils.GetId(c, "docId"),
	)

	body := isDocDeletedResponseBody{}
	if err == nil {
		body.Deleted = deleted
	}
	httpUtils.Respond(c, http.StatusOK, body, err)
}

type updateDocRequestBody struct {
	Lines   sharedTypes.Lines    `json:"lines"`
	Ranges  sharedTypes.Ranges   `json:"ranges"`
	Version *sharedTypes.Version `json:"version"`
}
type updateDocResponseBody struct {
	Revision sharedTypes.Revision `json:"rev"`
	Modified docstore.Modified    `json:"modified"`
}

func (h *httpController) updateDoc(c *gin.Context) {
	requestBody := &updateDocRequestBody{}
	if !httpUtils.MustParseJSON(requestBody, c) {
		return
	}
	if requestBody.Version == nil {
		httpUtils.RespondErr(c, &errors.ValidationError{
			Msg: "missing version",
		})
		return
	}
	modified, revision, err := h.dm.UpdateDoc(
		c,
		httpUtils.GetId(c, "projectId"),
		httpUtils.GetId(c, "docId"),
		requestBody.Lines,
		*requestBody.Version,
		requestBody.Ranges,
	)

	body := updateDocResponseBody{}
	if err == nil {
		body.Revision = revision
		body.Modified = modified
	}
	httpUtils.Respond(c, http.StatusOK, body, err)
}

func (h *httpController) patchDoc(c *gin.Context) {
	requestBody := &doc.Meta{}
	if !httpUtils.MustParseJSON(requestBody, c) {
		return
	}
	err := h.dm.PatchDoc(
		c,
		httpUtils.GetId(c, "projectId"),
		httpUtils.GetId(c, "docId"),
		*requestBody,
	)

	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) archiveDoc(c *gin.Context) {
	err := h.dm.ArchiveDoc(
		c,
		httpUtils.GetId(c, "projectId"),
		httpUtils.GetId(c, "docId"),
	)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}
