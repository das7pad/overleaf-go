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
	"crypto/subtle"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/httpUtils"
)

const (
	maxProxySize = 50 * 1024 * 1024
)

func newHttpController(timeout time.Duration, proxyToken string) httpController {
	return httpController{
		client: http.Client{
			Timeout: timeout,
		},
		proxyPathWithToken: "/proxy/" + proxyToken,
	}
}

type httpController struct {
	client             http.Client
	proxyPathWithToken string
}

func (h *httpController) GetRouter() http.Handler {
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/status", h.status)
	router.HEAD("/status", h.status)
	router.GET("/proxy/:token", h.proxy)
	return router
}

func (h *httpController) status(c *gin.Context) {
	c.String(http.StatusOK, "linked-url-proxy is alive (go)\n")
}

func (h *httpController) checkAuth(c *gin.Context) error {
	a := []byte(c.Request.URL.Path)
	b := []byte(h.proxyPathWithToken)
	if subtle.ConstantTimeCompare(a, b) == 1 {
		return nil
	}
	return &errors.NotAuthorizedError{}
}

func (h *httpController) proxy(c *gin.Context) {
	if err := h.checkAuth(c); err != nil {
		httpUtils.RespondErr(c, err)
		return
	}
	url := c.Query("url")
	if url == "" {
		httpUtils.RespondErr(c, &errors.ValidationError{Msg: "url missing"})
		return
	}

	request, err := http.NewRequestWithContext(
		c, http.MethodGet, url, http.NoBody,
	)
	if err != nil {
		httpUtils.RespondErr(c, errors.Tag(err, "request creation failed"))
		return
	}
	response, err := h.client.Do(request)
	if err != nil {
		httpUtils.RespondErr(c, errors.Tag(err, "request failed"))
		return
	}
	defer func() {
		_ = response.Body.Close()
	}()
	contentType := response.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if response.ContentLength > maxProxySize {
		httpUtils.RespondErr(c, &errors.BodyTooLargeError{})
		return
	}
	body := response.Body.(io.Reader)
	if response.ContentLength == -1 {
		body = io.LimitReader(response.Body, maxProxySize)
	}
	c.DataFromReader(
		response.StatusCode,
		response.ContentLength,
		contentType,
		body,
		map[string]string{
			"Content-Disposition": `attachment; filename="response"`,
		},
	)
}
