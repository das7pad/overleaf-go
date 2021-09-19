// Golang port of the Overleaf notifications service
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

	"github.com/das7pad/overleaf-go/pkg/httpUtils"
	"github.com/das7pad/overleaf-go/services/notifications/pkg/managers/notifications"
)

func newHttpController(cm notifications.Manager) httpController {
	return httpController{nm: cm}
}

type httpController struct {
	nm notifications.Manager
}

func (h *httpController) GetRouter(corsOptions httpUtils.CORSOptions,
	jwtOptions httpUtils.JWTOptions,

) http.Handler {
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/status", h.status)
	router.HEAD("/status", h.status)

	jwtRouter := router.Group("/jwt/notifications")
	jwtRouter.Use(httpUtils.CORS(corsOptions))
	jwtRouter.Use(httpUtils.NoCache())
	jwtRouter.Use(httpUtils.NewJWTHandler(jwtOptions).Middleware())
	jwtRouter.Use(httpUtils.ValidateAndSetJWTId("userId"))
	jwtRouter.GET("", h.getNotifications)
	jwtNotificationRouter := jwtRouter.Group("/:notificationId")
	jwtNotificationRouter.Use(httpUtils.ValidateAndSetId("notificationId"))
	jwtNotificationRouter.DELETE("", h.removeNotificationById)

	userRouter := router.Group("/user/:userId")
	userRouter.Use(httpUtils.ValidateAndSetId("userId"))
	userRouter.GET("", h.getNotifications)
	userRouter.POST("", h.addNotification)
	userRouter.DELETE("", h.removeNotificationByKey)

	userNotificationRouter := userRouter.Group("/notification/:notificationId")
	userNotificationRouter.Use(httpUtils.ValidateAndSetId("notificationId"))
	userNotificationRouter.DELETE("", h.removeNotificationById)

	router.DELETE("/key/:notificationKey", h.removeNotificationByKeyOnly)
	return router
}

func (h *httpController) status(c *gin.Context) {
	c.String(http.StatusOK, "notifications is alive (go)\n")
}

func (h *httpController) getNotifications(c *gin.Context) {
	n, err := h.nm.GetUserNotifications(
		c,
		httpUtils.GetId(c, "userId"),
	)
	httpUtils.Respond(c, http.StatusOK, n, err)
}

type addNotificationRequestBody struct {
	notifications.Notification
	ForceCreate bool `json:"forceCreate"`
}

func (h *httpController) addNotification(c *gin.Context) {
	requestBody := &addNotificationRequestBody{}
	if !httpUtils.MustParseJSON(requestBody, c) {
		return
	}

	err := h.nm.AddNotification(
		c,
		httpUtils.GetId(c, "userId"),
		requestBody.Notification,
		requestBody.ForceCreate,
	)
	httpUtils.Respond(c, http.StatusOK, nil, err)
}

func (h *httpController) removeNotificationById(c *gin.Context) {
	err := h.nm.RemoveNotificationById(
		c,
		httpUtils.GetId(c, "userId"),
		httpUtils.GetId(c, "notificationId"),
	)
	httpUtils.Respond(c, http.StatusOK, nil, err)
}

type removeNotificationByKeyRequestBody struct {
	Key string `json:"key"`
}

func (h *httpController) removeNotificationByKey(c *gin.Context) {
	requestBody := &removeNotificationByKeyRequestBody{}
	if !httpUtils.MustParseJSON(requestBody, c) {
		return
	}

	err := h.nm.RemoveNotificationByKey(
		c,
		httpUtils.GetId(c, "userId"),
		requestBody.Key,
	)
	httpUtils.Respond(c, http.StatusOK, nil, err)
}

func (h *httpController) removeNotificationByKeyOnly(c *gin.Context) {
	notificationKey := c.Param("notificationKey")
	err := h.nm.RemoveNotificationByKeyOnly(
		c,
		notificationKey,
	)
	httpUtils.Respond(c, http.StatusOK, nil, err)
}
