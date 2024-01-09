// Golang port of Overleaf
// Copyright (C) 2021-2024 Jakob Ackermann <das7pad@outlook.com>
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
	"context"
	"net/http"
	"net/netip"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/httpUtils"
	"github.com/das7pad/overleaf-go/pkg/options/env"
	"github.com/das7pad/overleaf-go/pkg/options/listenAddress"
)

var internalNetworks = strings.Join([]string{
	// Unspecified
	"0.0.0.0/32", "::/128",
	// Local
	"127.0.0.1/8", "::1/128",
	// Link-Local
	"169.254.0.0/16", "FE80::/10",
	// Private
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"FC00::/7",
	// CG NAT
	"100.64.0.0/10",
}, ",")

func main() {
	triggerExitCtx, triggerExit := signal.NotifyContext(
		context.Background(), syscall.SIGINT, syscall.SIGTERM,
	)
	defer triggerExit()

	timeout := env.GetDuration("LINKED_URL_PROXY_TIMEOUT", 28*time.Second)
	proxyToken := env.MustGetString("PROXY_TOKEN")
	allowRedirects := env.GetBool("ALLOW_REDIRECTS")
	var blockedNetworks []netip.Prefix
	{
		blockedRaw := env.GetString("BLOCKED_NETWORKS", internalNetworks)
		for _, s := range strings.Split(blockedRaw, ",") {
			b, err := netip.ParsePrefix(strings.TrimSpace(s))
			if err != nil {
				panic(errors.Tag(err, "parse CIDR: "+s))
			}
			blockedNetworks = append(blockedNetworks, b)
		}
	}
	handler := newHTTPController(
		timeout, proxyToken, allowRedirects, blockedNetworks,
	)

	eg, ctx := errgroup.WithContext(triggerExitCtx)
	server := http.Server{
		Handler: handler.GetRouter(),
	}
	eg.Go(func() error {
		return httpUtils.ListenAndServe(&server, listenAddress.Parse(8080))
	})
	eg.Go(func() error {
		<-ctx.Done()
		waitForSlowRequests, done := context.WithTimeout(
			context.Background(), timeout,
		)
		defer done()
		return server.Shutdown(waitForSlowRequests)
	})
	if err := eg.Wait(); err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}
