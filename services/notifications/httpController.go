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

	"github.com/das7pad/overleaf-go/pkg/httpUtils"
	"github.com/das7pad/overleaf-go/pkg/jwt/userIdJWT"
	"github.com/das7pad/overleaf-go/pkg/models/notification"
	"github.com/das7pad/overleaf-go/pkg/options/jwtOptions"
)

func newHttpController(cm notification.Manager) httpController {
	return httpController{nm: cm}
}

type httpController struct {
	nm notification.Manager
}

func (h *httpController) GetRouter(
	corsOptions httpUtils.CORSOptions,
	jwtOptions jwtOptions.JWTOptions,
) http.Handler {
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/status", h.status)
	router.HEAD("/status", h.status)

	jwtRouter := router.Group("/jwt/notifications")
	jwtRouter.Use(httpUtils.CORS(corsOptions))
	jwtRouter.Use(httpUtils.NoCache())
	jwtRouter.Use(
		httpUtils.NewJWTHandler(userIdJWT.New(jwtOptions)).Middleware(),
	)
	jwtRouter.GET("", h.getNotifications)
	jwtNotificationRouter := jwtRouter.Group("/:notificationId")
	jwtNotificationRouter.Use(httpUtils.ValidateAndSetId("notificationId"))
	jwtNotificationRouter.DELETE("", h.removeNotificationById)
	return router
}

func (h *httpController) status(c *gin.Context) {
	c.String(http.StatusOK, "notifications is alive (go)\n")
}

func (h *httpController) getNotifications(c *gin.Context) {
	notifications := make([]notification.Notification, 0)
	err := h.nm.GetAllForUser(
		c,
		httpUtils.GetId(c, "userId"),
		&notifications,
	)
	httpUtils.Respond(c, http.StatusOK, notifications, err)
}

func (h *httpController) removeNotificationById(c *gin.Context) {
	err := h.nm.RemoveById(
		c,
		httpUtils.GetId(c, "userId"),
		httpUtils.GetId(c, "notificationId"),
	)
	httpUtils.Respond(c, http.StatusOK, nil, err)
}
