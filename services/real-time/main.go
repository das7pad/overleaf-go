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

	"github.com/das7pad/overleaf-go/cmd/pkg/utils"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/httpUtils"
	"github.com/das7pad/overleaf-go/pkg/options/env"
	"github.com/das7pad/overleaf-go/pkg/options/listenAddress"
	"github.com/das7pad/overleaf-go/pkg/pendingOperation"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater"
	documentUpdaterTypes "github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/managers/realTime"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/router"
	realTimeTypes "github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

func main() {
	triggerExitCtx, triggerExit := signal.NotifyContext(
		context.Background(), syscall.SIGINT, syscall.SIGTERM,
	)
	defer triggerExit()

	rClient := utils.MustConnectRedis(triggerExitCtx)
	db := utils.MustConnectPostgres(triggerExitCtx)

	dumOptions := documentUpdaterTypes.Options{}
	dumOptions.FillFromEnv()
	dum, err := documentUpdater.New(&dumOptions, db, rClient)
	if err != nil {
		panic(errors.Tag(err, "document-updater setup"))
	}

	realTimeOptions := realTimeTypes.Options{}
	realTimeOptions.FillFromEnv()
	rtm, err := realTime.New(
		context.Background(),
		&realTimeOptions,
		db,
		rClient,
		dum,
	)
	if err != nil {
		panic(errors.Tag(err, "realTime setup"))
	}

	eg, ctx := errgroup.WithContext(triggerExitCtx)
	eg.Go(func() error {
		rtm.PeriodicCleanup(ctx)
		return nil
	})

	var server httpUtils.Server
	if env.GetBool("USE_WS_SERVER") {
		srv := router.WS(rtm, &realTimeOptions)
		server = srv
		eg.Go(func() error {
			<-ctx.Done()
			srv.SetStatus(false)
			return nil
		})
	} else {
		server = &http.Server{Handler: router.New(rtm, &realTimeOptions)}
	}
	httpUtils.ListenAndServeEach(eg.Go, server, listenAddress.Parse(3026))
	eg.Go(func() error {
		<-ctx.Done()
		rtm.InitiateGracefulShutdown()
		ctx2, done := context.WithTimeout(context.Background(), 15*time.Second)
		defer done()
		pendingShutdown := pendingOperation.TrackOperation(func() error {
			return server.Shutdown(ctx2)
		})
		rtm.TriggerGracefulReconnect()
		err2 := pendingShutdown.Wait(ctx2)
		rtm.DisconnectAll()
		dum.WaitForBackgroundFlush()
		return err2
	})
	if err = eg.Wait(); err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}
