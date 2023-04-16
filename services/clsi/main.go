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
	"context"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/options/env"
	"github.com/das7pad/overleaf-go/pkg/options/listenAddress"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/loadAgent"
	clsiTypes "github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

func main() {
	triggerExitCtx, triggerExit := signal.NotifyContext(
		context.Background(), syscall.SIGINT, syscall.SIGTERM,
	)
	defer triggerExit()

	clsiOptions := clsiTypes.Options{}
	clsiOptions.FillFromEnv()
	clsiManager, err := clsi.New(&clsiOptions)
	if err != nil {
		panic(errors.Tag(err, "clsi setup"))
	}

	loadAgentSocket, err := loadAgent.Start(
		listenAddress.Parse(env.GetInt("LOAD_PORT", 3048)),
		env.GetBool("LOAD_SHEDDING"),
		env.GetDuration("LOAD_REFRESH_CAPACITY_EVERY", 3*time.Second),
	)
	if err != nil {
		panic(errors.Tag(err, "load agent setup"))
	}

	eg, ctx := errgroup.WithContext(triggerExitCtx)
	eg.Go(func() error {
		clsiManager.PeriodicCleanup(ctx)
		return nil
	})

	server := http.Server{
		Addr:    listenAddress.Parse(3013),
		Handler: newHTTPController(clsiManager).GetRouter(),
	}
	eg.Go(func() error {
		return server.ListenAndServe()
	})
	eg.Go(func() error {
		<-ctx.Done()
		_ = loadAgentSocket.Close()
		waitForSlowRequests, done := context.WithTimeout(
			context.Background(), time.Second*30,
		)
		defer done()
		return server.Shutdown(waitForSlowRequests)
	})
	if err = eg.Wait(); err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}
