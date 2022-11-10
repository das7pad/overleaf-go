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
	"context"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/cmd/pkg/utils"
	"github.com/das7pad/overleaf-go/pkg/errors"
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
		context.Background(), syscall.SIGINT, syscall.SIGUSR1, syscall.SIGTERM,
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
		rtm.PeriodicCleanup(triggerExitCtx)
		return nil
	})
	server := http.Server{
		Addr:    listenAddress.Parse(3026),
		Handler: router.New(rtm, realTimeOptions.JWT.RealTime),
	}
	var errServe error
	eg.Go(func() error {
		errServe = server.ListenAndServe()
		triggerExit()
		if errServe == http.ErrServerClosed {
			errServe = nil
		}
		return errServe
	})
	eg.Go(func() error {
		<-ctx.Done()
		rtm.InitiateGracefulShutdown()
		ctx2, done := context.WithTimeout(context.Background(), 15*time.Second)
		defer done()
		pendingShutdown := pendingOperation.TrackOperation(func() error {
			return server.Shutdown(ctx2)
		})
		rtm.TriggerGracefulReconnect()
		return pendingShutdown.Wait(ctx2)
	})
	err = eg.Wait()
	if errServe != nil {
		panic(errServe)
	}
	if err != nil {
		panic(err)
	}
}
