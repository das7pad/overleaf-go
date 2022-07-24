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
	"net/http"

	"github.com/das7pad/overleaf-go/pkg/httpUtils"
	"github.com/das7pad/overleaf-go/services/spelling/pkg/managers/spelling"
	"github.com/das7pad/overleaf-go/services/spelling/pkg/types"
)

func New(cm spelling.Manager, corsOptions httpUtils.CORSOptions) *httpUtils.Router {
	router := httpUtils.NewRouter(&httpUtils.RouterOptions{})
	Add(router, cm, corsOptions)
	return router
}

func Add(r *httpUtils.Router, cm spelling.Manager, corsOptions httpUtils.CORSOptions) {
	(&httpController{sm: cm}).addRoutes(r, corsOptions)
}

type httpController struct {
	sm spelling.Manager
}

func (h *httpController) addRoutes(router *httpUtils.Router, corsOptions httpUtils.CORSOptions) {
	r := router.Group("")
	r.Use(httpUtils.CORS(corsOptions))
	r.POST("/spelling/api/check", h.check)
}

type checkRequestBody struct {
	Language types.SpellCheckLanguage `json:"language"`
	Words    []string                 `json:"words"`
}

type checkResponseBody struct {
	Misspellings []types.Misspelling `json:"misspellings"`
}

func (h *httpController) check(c *httpUtils.Context) {
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
