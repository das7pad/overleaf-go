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
	"github.com/das7pad/overleaf-go/services/spelling/pkg/managers/spelling"
	"github.com/das7pad/overleaf-go/services/spelling/pkg/types"
)

func newHttpController(cm spelling.Manager) httpController {
	return httpController{sm: cm}
}

type httpController struct {
	sm spelling.Manager
}

func (h *httpController) GetRouter(
	corsOptions httpUtils.CORSOptions,
	jwtOptions httpUtils.JWTOptions,
) http.Handler {
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/status", h.status)
	router.HEAD("/status", h.status)

	jwtRouter := router.Group("/jwt/spelling/v20200714")
	jwtRouter.Use(httpUtils.CORS(corsOptions))
	jwtRouter.Use(httpUtils.NoCache())
	jwtRouter.Use(httpUtils.NewJWTHandler(jwtOptions).Middleware())
	jwtRouter.Use(httpUtils.ValidateAndSetJWTId("userId"))
	jwtRouter.POST("/check", h.check)
	jwtRouter.GET("/dict", h.getDictionary)
	jwtRouter.GET("/learn", h.learn)
	jwtRouter.GET("/unlearn", h.unlearn)

	prefixes := []string{"", "/v20200714"}
	for _, prefix := range prefixes {
		router.POST(prefix+"/check", h.check)

		userRouter := router.Group(prefix + "/user/:userId")
		userRouter.Use(httpUtils.ValidateAndSetId("userId"))
		userRouter.DELETE("", h.deleteDictionary)
		userRouter.GET("", h.getDictionary)
		userRouter.POST("/check", h.check)
		userRouter.DELETE("/dict", h.getDictionary)
		userRouter.POST("/learn", h.learn)
		userRouter.POST("/unlearn", h.unlearn)
	}
	return router
}

func (h *httpController) status(c *gin.Context) {
	c.String(http.StatusOK, "spelling is alive (go)\n")
}

type checkRequestBody struct {
	Language string   `json:"language"`
	Words    []string `json:"words"`
}

type checkResponseBody struct {
	Misspellings []types.Misspelling `json:"misspellings"`
}

func (h *httpController) check(c *gin.Context) {
	requestBody := &checkRequestBody{}
	if !httpUtils.MustParseJSON(requestBody, c) {
		return
	}
	misspellings, err := h.sm.CheckWords(
		c,
		requestBody.Language,
		requestBody.Words,
	)
	responseBody := checkResponseBody{Misspellings: misspellings}
	httpUtils.Respond(c, http.StatusOK, responseBody, err)
}

func (h *httpController) deleteDictionary(c *gin.Context) {
	err := h.sm.DeleteDictionary(
		c,
		httpUtils.GetId(c, "userId"),
	)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) getDictionary(c *gin.Context) {
	dictionary, err := h.sm.GetDictionary(
		c,
		httpUtils.GetId(c, "userId"),
	)
	httpUtils.Respond(c, http.StatusOK, dictionary, err)
}

type learnRequestBody struct {
	Word string `json:"word"`
}

func (h *httpController) learn(c *gin.Context) {
	requestBody := &learnRequestBody{}
	if !httpUtils.MustParseJSON(requestBody, c) {
		return
	}
	err := h.sm.LearnWord(
		c,
		httpUtils.GetId(c, "userId"),
		requestBody.Word,
	)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}

func (h *httpController) unlearn(c *gin.Context) {
	requestBody := &learnRequestBody{}
	if !httpUtils.MustParseJSON(requestBody, c) {
		return
	}
	err := h.sm.UnlearnWord(
		c,
		httpUtils.GetId(c, "userId"),
		requestBody.Word,
	)
	httpUtils.Respond(c, http.StatusNoContent, nil, err)
}
