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
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/overleaf-go/pkg/httpUtils"
	"github.com/das7pad/overleaf-go/pkg/models/contact"
)

func newHttpController(cm contact.Manager) httpController {
	return httpController{cm: cm}
}

type httpController struct {
	cm contact.Manager
}

func (h *httpController) GetRouter() http.Handler {
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/status", h.status)
	router.HEAD("/status", h.status)
	userRouter := router.Group("/user/:userId")
	userRouter.Use(httpUtils.ValidateAndSetId("userId"))
	userRouter.GET("/contacts", h.getContacts)
	userRouter.POST("/contacts", h.addContacts)

	return router
}

func (h *httpController) status(c *gin.Context) {
	c.String(http.StatusOK, "contacts is alive (go)\n")
}

type addContactRequestBody struct {
	ContactId primitive.ObjectID `json:"contact_id"`
}

func (h *httpController) addContacts(c *gin.Context) {
	userId := httpUtils.GetId(c, "userId")
	requestBody := &addContactRequestBody{}
	if !httpUtils.MustParseJSON(requestBody, c) {
		return
	}
	err := h.cm.Add(c, userId, requestBody.ContactId)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

type getContactsResponseBody struct {
	ContactIds []primitive.ObjectID `json:"contact_ids"`
}

func (h *httpController) getContacts(c *gin.Context) {
	userId := httpUtils.GetId(c, "userId")

	contactIds := make([]primitive.ObjectID, 0)
	err := h.cm.GetForUser(c, userId, &contactIds)
	body := &getContactsResponseBody{ContactIds: contactIds}
	httpUtils.Respond(c, http.StatusOK, body, err)
}
