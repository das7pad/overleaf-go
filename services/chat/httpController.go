// Golang port of the Overleaf chat service
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
	"log"
	"net/http"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/httpUtils"
	"github.com/das7pad/overleaf-go/services/chat/pkg/managers/chat"
)

func newHttpController(cm chat.Manager) httpController {
	return httpController{cm: cm}
}

type httpController struct {
	cm chat.Manager
}

func (h *httpController) GetRouter() http.Handler {
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/status", h.status)
	projectRouter := router.Group("/project/:projectId")
	projectRouter.Use(httpUtils.ValidateAndSetId("projectId"))

	threadRouter := projectRouter.Group("/thread/:threadId")
	threadRouter.Use(httpUtils.ValidateAndSetId("threadId"))

	threadMessagesRouter := threadRouter.Group("/messages/:messageId")
	threadMessagesRouter.Use(httpUtils.ValidateAndSetId("messageId"))

	projectRouter.GET("/messages", h.getGlobalMessages)
	projectRouter.POST("/messages", h.sendGlobalMessages)
	projectRouter.GET("/threads", h.getAllThreads)

	threadRouter.POST("/messages", h.sendThreadMessage)
	threadRouter.POST("/resolve", h.resolveThread)
	threadRouter.POST("/reopen", h.reopenThread)
	threadRouter.DELETE("", h.deleteThread)

	threadMessagesRouter.POST("/edit", h.editMessage)
	threadMessagesRouter.DELETE("", h.deleteMessage)

	return router
}

func errorResponse(c *gin.Context, code int, message string) {
	// Align the error messages with the NodeJS implementation/tests.
	if message == "invalid payload" {
		// Realistically only the user_id field is of interest.
		message = "invalid user_id"
	}
	// Emit a capitalized error message.
	message = fmt.Sprintf(
		"%s%s",
		string(unicode.ToTitle(rune(message[0]))),
		message[1:],
	)
	// Report errors in route parameter validation as projectId -> project_id.
	message = strings.ReplaceAll(message, "Id", "_id")

	// Flush it and ignore any errors.
	c.String(code, message)
}

func (h *httpController) status(c *gin.Context) {
	c.String(http.StatusOK, "chat is alive (go)\n")
}

func respond(
	c *gin.Context,
	code int,
	body interface{},
	err error,
	msg string,
) {
	if err != nil {
		if errors.IsValidationError(err) {
			errorResponse(c, http.StatusBadRequest, err.Error())
			return
		}
		log.Printf("%s %s: %s: %v", c.Request.Method, c.Request.URL.Path, msg, err)
		errorResponse(c, http.StatusInternalServerError, msg)
		return
	}
	if body != nil {
		c.JSON(code, body)
	} else {
		c.Status(code)
	}
}

const defaultMessageLimit = 50

type getGlobalMessagesRequestOptions struct {
	Limit  int64   `form:"limit"`
	Before float64 `form:"before"`
}

func (h *httpController) getGlobalMessages(c *gin.Context) {
	var options getGlobalMessagesRequestOptions
	if c.MustBindWith(&options, binding.Query) != nil {
		return
	}
	if options.Limit == 0 {
		options.Limit = defaultMessageLimit
	}

	messages, err := h.cm.GetGlobalMessages(
		c,
		httpUtils.GetId(c, "projectId"),
		options.Limit,
		options.Before,
	)
	respond(c, http.StatusOK, messages, err, "cannot get global messages")
}

type sendMessageRequestBody struct {
	Content string             `json:"content"`
	UserId  primitive.ObjectID `json:"user_id"`
}

func (h *httpController) sendGlobalMessages(c *gin.Context) {
	var requestBody sendMessageRequestBody
	if c.MustBindWith(&requestBody, binding.JSON) != nil {
		return
	}
	message, err := h.cm.SendGlobalMessage(
		c,
		httpUtils.GetId(c, "projectId"),
		requestBody.Content,
		requestBody.UserId,
	)
	respond(c, http.StatusCreated, message, err, "cannot send global message")
}

func (h *httpController) getAllThreads(c *gin.Context) {
	threads, err := h.cm.GetAllThreads(
		c,
		httpUtils.GetId(c, "projectId"),
	)
	respond(c, http.StatusOK, threads, err, "cannot get all threads")
}

func (h *httpController) sendThreadMessage(c *gin.Context) {
	var requestBody sendMessageRequestBody
	if c.MustBindWith(&requestBody, binding.JSON) != nil {
		return
	}
	message, err := h.cm.SendThreadMessage(
		c,
		httpUtils.GetId(c, "projectId"),
		httpUtils.GetId(c, "threadId"),
		requestBody.Content,
		requestBody.UserId,
	)
	respond(c, http.StatusCreated, message, err, "cannot send thread message")
}

type resolveThreadRequestBody struct {
	UserId primitive.ObjectID `json:"user_id"`
}

func (h *httpController) resolveThread(c *gin.Context) {
	var requestBody resolveThreadRequestBody
	if c.MustBindWith(&requestBody, binding.JSON) != nil {
		return
	}
	err := h.cm.ResolveThread(
		c,
		httpUtils.GetId(c, "projectId"),
		httpUtils.GetId(c, "threadId"),
		requestBody.UserId,
	)
	respond(c, http.StatusNoContent, nil, err, "cannot resolve thread")
}

func (h *httpController) reopenThread(c *gin.Context) {
	err := h.cm.ReopenThread(
		c,
		httpUtils.GetId(c, "projectId"),
		httpUtils.GetId(c, "threadId"),
	)
	respond(c, http.StatusNoContent, nil, err, "cannot reopen thread")
}

func (h *httpController) deleteThread(c *gin.Context) {
	err := h.cm.DeleteThread(
		c,
		httpUtils.GetId(c, "projectId"),
		httpUtils.GetId(c, "threadId"),
	)
	respond(c, http.StatusNoContent, nil, err, "cannot delete thread")
}

type editMessageRequestBody struct {
	Content string `json:"content"`
}

func (h *httpController) editMessage(c *gin.Context) {
	var requestBody editMessageRequestBody
	if c.MustBindWith(&requestBody, binding.JSON) != nil {
		return
	}
	err := h.cm.EditMessage(
		c,
		httpUtils.GetId(c, "projectId"),
		httpUtils.GetId(c, "threadId"),
		httpUtils.GetId(c, "messageId"),
		requestBody.Content,
	)
	respond(c, http.StatusNoContent, nil, err, "cannot edit message")
}

func (h *httpController) deleteMessage(c *gin.Context) {
	err := h.cm.DeleteMessage(
		c,
		httpUtils.GetId(c, "projectId"),
		httpUtils.GetId(c, "threadId"),
		httpUtils.GetId(c, "messageId"),
	)
	respond(c, http.StatusNoContent, nil, err, "cannot delete message")
}
