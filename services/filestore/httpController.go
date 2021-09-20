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
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/httpUtils"
	"github.com/das7pad/overleaf-go/pkg/objectStorage"
	"github.com/das7pad/overleaf-go/services/filestore/pkg/managers/filestore"
)

func newHttpController(fm filestore.Manager, allowRedirects bool) httpController {
	return httpController{fm: fm, allowRedirects: allowRedirects}
}

type httpController struct {
	fm             filestore.Manager
	allowRedirects bool
}

func (h *httpController) GetRouter() http.Handler {
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/status", h.status)
	router.HEAD("/status", h.status)

	projectRouter := router.Group("/project/:projectId")
	projectRouter.Use(httpUtils.ValidateAndSetId("projectId"))
	projectFileRouter := projectRouter.Group("/file/:fileId")
	projectFileRouter.Use(httpUtils.ValidateAndSetId("fileId"))

	projectRouter.DELETE("", h.deleteProject)
	projectRouter.GET("/size", h.getProjectSize)

	projectFileRouter.DELETE("", h.deleteProjectFile)
	projectFileRouter.GET("", h.getProjectFile)
	projectFileRouter.HEAD("", h.getProjectFileHEAD)
	projectFileRouter.POST("", h.sendProjectFile)
	projectFileRouter.PUT("", h.copyProjectFile)
	projectFileRouter.PUT("/upload", h.sendProjectFileViaPUT)
	projectFileRouter.PUT("/copy", h.copyProjectFile)
	return router
}

func redirect(
	c *gin.Context,
	u *url.URL,
	err error,
) {
	if err == nil {
		c.Redirect(http.StatusTemporaryRedirect, u.String())
	}
	httpUtils.Respond(c, http.StatusTemporaryRedirect, nil, err)
}

func (h *httpController) status(c *gin.Context) {
	c.String(http.StatusOK, "filestore is alive (go)\n")
}

func (h *httpController) deleteProject(c *gin.Context) {
	err := h.fm.DeleteProject(
		c,
		httpUtils.GetId(c, "projectId"),
	)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

type getProjectSizeResponseBody struct {
	TotalSize int64 `json:"total bytes"`
}

func (h *httpController) getProjectSize(c *gin.Context) {
	s, err := h.fm.GetSizeOfProject(
		c,
		httpUtils.GetId(c, "projectId"),
	)
	body := getProjectSizeResponseBody{TotalSize: s}
	httpUtils.Respond(c, http.StatusOK, body, err)
}

func (h *httpController) deleteProjectFile(c *gin.Context) {
	err := h.fm.DeleteProjectFile(
		c,
		httpUtils.GetId(c, "projectId"),
		httpUtils.GetId(c, "fileId"),
	)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) getProjectFile(c *gin.Context) {
	if h.allowRedirects {
		u, err := h.fm.GetRedirectURLForGETOnProjectFile(
			c,
			httpUtils.GetId(c, "projectId"),
			httpUtils.GetId(c, "fileId"),
		)
		redirect(c, u, err)
		return
	}
	options := objectStorage.GetOptions{}
	body, err := h.fm.GetReadStreamForProjectFile(
		c,
		httpUtils.GetId(c, "projectId"),
		httpUtils.GetId(c, "fileId"),
		options,
	)
	if err != nil {
		httpUtils.Respond(c, http.StatusOK, nil, err)
		return
	}
	c.Header("Content-Type", "application/octet-stream")
	_, _ = io.Copy(c.Writer, body)
}

func (h *httpController) getProjectFileHEAD(c *gin.Context) {
	if h.allowRedirects {
		u, err := h.fm.GetRedirectURLForHEADOnProjectFile(
			c,
			httpUtils.GetId(c, "projectId"),
			httpUtils.GetId(c, "fileId"),
		)
		redirect(c, u, err)
		return
	}
	size, err := h.fm.GetSizeOfProjectFile(
		c,
		httpUtils.GetId(c, "projectId"),
		httpUtils.GetId(c, "fileId"),
	)
	if err == nil {
		c.Header("Content-Length", strconv.FormatInt(size, 10))
	}
	httpUtils.Respond(c, http.StatusOK, nil, err)
}

func (h *httpController) sendProjectFile(c *gin.Context) {
	// NOTE: This is a POST request. We cannot redirect to a PUT URL.
	//       Redirecting to a singed POST URL does not work unless additional
	//        POST form data is amended.
	//       This needs an API rework for uploading via PUT.

	options := objectStorage.SendOptions{
		ContentSize:     c.Request.ContentLength,
		ContentEncoding: c.GetHeader("Content-Encoding"),
		ContentType:     c.GetHeader("Content-Type"),
	}
	err := h.fm.SendStreamForProjectFile(
		c,
		httpUtils.GetId(c, "projectId"),
		httpUtils.GetId(c, "fileId"),
		c.Request.Body,
		options,
	)
	httpUtils.Respond(c, http.StatusOK, nil, err)
}

func (h *httpController) sendProjectFileViaPUT(c *gin.Context) {
	if h.allowRedirects {
		u, err := h.fm.GetRedirectURLForPUTOnProjectFile(
			c,
			httpUtils.GetId(c, "projectId"),
			httpUtils.GetId(c, "fileId"),
		)
		redirect(c, u, err)
		return
	}

	options := objectStorage.SendOptions{
		ContentSize:     c.Request.ContentLength,
		ContentEncoding: c.GetHeader("Content-Encoding"),
		ContentType:     c.GetHeader("Content-Type"),
	}
	err := h.fm.SendStreamForProjectFile(
		c,
		httpUtils.GetId(c, "projectId"),
		httpUtils.GetId(c, "fileId"),
		c.Request.Body,
		options,
	)
	httpUtils.Respond(c, http.StatusOK, nil, err)
}

type copyProjectRequestBody struct {
	Source struct {
		ProjectId primitive.ObjectID `json:"project_id" binding:"required"`
		FileId    primitive.ObjectID `json:"file_id" binding:"required"`
	} `json:"source" binding:"required"`
}

func (h *httpController) copyProjectFile(c *gin.Context) {
	requestBody := &copyProjectRequestBody{}
	if !httpUtils.MustParseJSON(requestBody, c) {
		return
	}
	err := h.fm.CopyProjectFile(
		c,
		requestBody.Source.ProjectId,
		requestBody.Source.FileId,
		httpUtils.GetId(c, "projectId"),
		httpUtils.GetId(c, "fileId"),
	)
	httpUtils.Respond(c, http.StatusOK, nil, err)
}
