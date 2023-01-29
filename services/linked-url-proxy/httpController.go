// Golang port of Overleaf
// Copyright (C) 2021-2023 Jakob Ackermann <das7pad@outlook.com>
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
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/httpUtils"
	"github.com/das7pad/overleaf-go/services/linked-url-proxy/pkg/constants"
)

const (
	maxProxySize = 50 * 1024 * 1024
)

var (
	clientErrors = []string{
		"blocked redirect",
		"ip blocked",
		"connection refused",
		"no such host",
	}
)

func newHTTPController(timeout time.Duration, proxyToken string, allowRedirects bool, blockedNetworks []netip.Prefix) httpController {
	checkRedirect := func(_ *http.Request, _ []*http.Request) error {
		return &errors.UnprocessableEntityError{Msg: "blocked redirect"}
	}
	if allowRedirects {
		checkRedirect = nil
	}
	return httpController{
		client: http.Client{
			Timeout:       timeout,
			CheckRedirect: checkRedirect,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				IdleConnTimeout:     timeout,
				TLSHandshakeTimeout: 10 * time.Second,
				DialContext: (&net.Dialer{
					Timeout:   timeout,
					KeepAlive: timeout,
					Control: func(_, addr string, _ syscall.RawConn) error {
						a, err := netip.ParseAddrPort(addr)
						if err != nil {
							return &errors.ValidationError{Msg: err.Error()}
						}
						ip := a.Addr()
						for _, b := range blockedNetworks {
							if b.Contains(ip) {
								return &errors.ValidationError{
									Msg: "ip blocked",
								}
							}
						}
						return nil
					},
				}).DialContext,
			},
		},
		proxyPathWithToken: "/proxy/" + proxyToken,
	}
}

type httpController struct {
	client             http.Client
	proxyPathWithToken string
}

func (h *httpController) GetRouter() http.Handler {
	router := httpUtils.NewRouter(&httpUtils.RouterOptions{})
	router.GET("/proxy/{token}", h.proxy)
	return router
}

func (h *httpController) checkAuth(c *httpUtils.Context) error {
	a := []byte(c.Request.URL.Path)
	b := []byte(h.proxyPathWithToken)
	if subtle.ConstantTimeCompare(a, b) == 1 {
		return nil
	}
	return &errors.NotAuthorizedError{}
}

func (h *httpController) proxy(c *httpUtils.Context) {
	c.Writer.Header().Add("Via", constants.LinkedUrlProxy)
	if err := h.checkAuth(c); err != nil {
		httpUtils.RespondErr(c, err)
		return
	}
	q := c.Request.URL.Query()
	url := q.Get(constants.QueryNameURL)
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
		msg := err.Error()
		for _, clientErr := range clientErrors {
			if strings.HasSuffix(msg, clientErr) {
				err = &errors.ValidationError{Msg: clientErr}
				break
			}
		}
		httpUtils.RespondErr(c, errors.Tag(err, "request failed"))
		return
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if response.ContentLength > maxProxySize {
		httpUtils.RespondErr(c, &errors.BodyTooLargeError{})
		return
	}
	if statusCode := response.StatusCode; statusCode != http.StatusOK {
		via := response.Header.Get("Via")
		if q.Get(constants.QueryNameProxyChainMarker) == "true" &&
			strings.Contains(via, constants.LinkedUrlProxy) {
			cloneHeaders := []string{
				"Via", "X-Served-By", "X-Request-Id",
				"CF-RAY", "Fly-Request-Id", "Function-Execution-Id",
				constants.HeaderXUpstreamStatusCode,
			}
			c.Writer.Header().Del("Via") // move to the end
			for _, name := range cloneHeaders {
				for _, v := range response.Header.Values(name) {
					c.Writer.Header().Add(name, v)
				}
			}
			c.Writer.Header().Add("Via", constants.LinkedUrlProxy)
			c.Writer.WriteHeader(statusCode)
			_, _ = io.Copy(c.Writer, response.Body)
			return
		}
		s := strconv.FormatInt(int64(statusCode), 10)
		c.Writer.Header().Set(constants.HeaderXUpstreamStatusCode, s)
		httpUtils.RespondErr(c, &errors.UnprocessableEntityError{
			Msg: "upstream responded with " + s,
		})
		return
	}
	body := response.Body.(io.Reader)
	if response.ContentLength == -1 {
		body = io.LimitReader(response.Body, maxProxySize)
	}
	if err = c.Err(); err != nil {
		httpUtils.RespondErr(c, err)
		return
	}
	if response.ContentLength != -1 {
		// Not used by client, but used by std lib for identifying de-sync.
		c.Writer.Header().Set(
			"Content-Length",
			strconv.FormatInt(response.ContentLength, 10),
		)
	}
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = io.Copy(c.Writer, body)
}
