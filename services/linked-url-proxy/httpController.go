// Golang port of the Overleaf linked-url-proxy service
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
	"log"
	"net/http"
	"time"
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
	mux := http.NewServeMux()
	mux.HandleFunc("/status", h.status)
	mux.HandleFunc("/proxy/", h.proxy)
	return mux
}

func (h *httpController) status(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("linked-url-proxy is alive (go)\n"))
}

func (h *httpController) proxy(w http.ResponseWriter, r *http.Request) {
	if subtle.ConstantTimeCompare([]byte(r.URL.Path), []byte(h.proxyPathWithToken)) == 0 {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	url := r.URL.Query().Get("url")
	if url == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	requestOut, err := http.NewRequestWithContext(r.Context(), "GET", url, http.NoBody)
	if err != nil {
		log.Println("request creation failed:", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	responseOut, err := h.client.Do(requestOut)
	if err != nil {
		log.Println("request failed:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	contentType := responseOut.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Add("Content-Type", contentType)
	w.Header().Add("Content-Disposition", "attachment; filename=\"response\"")
	w.WriteHeader(responseOut.StatusCode)

	_, err = io.Copy(w, responseOut.Body)
	if err != nil {
		log.Println("proxy failed:", err)
	}
}
