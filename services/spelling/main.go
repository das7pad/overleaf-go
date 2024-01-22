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
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/httpUtils"
	"github.com/das7pad/overleaf-go/pkg/options/corsOptions"
	"github.com/das7pad/overleaf-go/pkg/options/listenAddress"
	"github.com/das7pad/overleaf-go/services/spelling/pkg/managers/spelling"
	"github.com/das7pad/overleaf-go/services/spelling/pkg/router"
	spellingTypes "github.com/das7pad/overleaf-go/services/spelling/pkg/types"
)

func main() {
	triggerExitCtx, triggerExit := signal.NotifyContext(
		context.Background(), syscall.SIGINT, syscall.SIGTERM,
	)
	defer triggerExit()

	spellingOptions := spellingTypes.Options{}
	spellingOptions.FillFromEnv()
	sm, err := spelling.New(&spellingOptions)
	if err != nil {
		panic(errors.Tag(err, "spelling setup"))
	}

	eg, ctx := errgroup.WithContext(triggerExitCtx)
	server := http.Server{
		Handler: router.New(sm, corsOptions.Parse()),
	}
	httpUtils.ListenAndServeEach(eg.Go, &server, listenAddress.Parse(3005))
	eg.Go(func() error {
		<-ctx.Done()
		waitForSlowRequests, done := context.WithTimeout(
			context.Background(), time.Second*10,
		)
		defer done()
		return server.Shutdown(waitForSlowRequests)
	})
	if err = eg.Wait(); err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}
