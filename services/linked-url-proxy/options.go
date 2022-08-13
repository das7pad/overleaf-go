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
	"time"

	"github.com/das7pad/overleaf-go/pkg/options/listenAddress"
	"github.com/das7pad/overleaf-go/pkg/options/utils"
)

type linkedURLProxyOptions struct {
	address        string
	timeout        time.Duration
	proxyToken     string
	allowRedirects bool
}

func getOptions() *linkedURLProxyOptions {
	o := &linkedURLProxyOptions{}
	o.address = listenAddress.Parse(8080)
	o.proxyToken = utils.MustGetStringFromEnv("PROXY_TOKEN")
	o.timeout = utils.GetDurationFromEnv(
		"LINKED_URL_PROXY_TIMEOUT", 28*time.Second,
	)
	o.allowRedirects = utils.GetStringFromEnv(
		"ALLOW_REDIRECTS", "false",
	) == "true"
	return o
}
