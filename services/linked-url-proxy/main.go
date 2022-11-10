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

package main

import (
	"net/http"
	"time"

	"github.com/das7pad/overleaf-go/pkg/options/env"
	"github.com/das7pad/overleaf-go/pkg/options/listenAddress"
)

func main() {
	timeout := env.GetDuration("LINKED_URL_PROXY_TIMEOUT", 28*time.Second)
	proxyToken := env.MustGetString("PROXY_TOKEN")
	allowRedirects := env.GetBool("ALLOW_REDIRECTS")
	handler := newHTTPController(timeout, proxyToken, allowRedirects)
	server := http.Server{
		Addr:    listenAddress.Parse(8080),
		Handler: handler.GetRouter(),
	}
	err := server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}
