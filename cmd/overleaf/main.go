// Golang port of Overleaf
// Copyright (C) 2022-2023 Jakob Ackermann <das7pad@outlook.com>
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
	"strings"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/cmd/pkg/utils"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/httpUtils"
	"github.com/das7pad/overleaf-go/pkg/options/corsOptions"
	"github.com/das7pad/overleaf-go/pkg/options/listenAddress"
	"github.com/das7pad/overleaf-go/pkg/pendingOperation"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi"
	clsiTypes "github.com/das7pad/overleaf-go/services/clsi/pkg/types"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater"
	documentUpdaterTypes "github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/managers/realTime"
	realTimeRouter "github.com/das7pad/overleaf-go/services/real-time/pkg/router"
	realTimeTypes "github.com/das7pad/overleaf-go/services/real-time/pkg/types"
	"github.com/das7pad/overleaf-go/services/spelling/pkg/managers/spelling"
	spellingRouter "github.com/das7pad/overleaf-go/services/spelling/pkg/router"
	spellingTypes "github.com/das7pad/overleaf-go/services/spelling/pkg/types"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web"
	webRouter "github.com/das7pad/overleaf-go/services/web/pkg/router"
	webTypes "github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func main() {
	triggerExitCtx, triggerExit := signal.NotifyContext(
		context.Background(), syscall.SIGINT, syscall.SIGTERM,
	)
	defer triggerExit()

	rClient := utils.MustConnectRedis(triggerExitCtx)
	db := utils.MustConnectPostgres(triggerExitCtx)
	addr := listenAddress.Parse(3000)
	localURL := "http://" + addr
	if strings.HasPrefix(addr, ":") || strings.HasPrefix(addr, "0.0.0.0") {
		localURL = "http://127.0.0.1" + strings.TrimPrefix(addr, "0.0.0.0")
	}

	clsiOptions := clsiTypes.Options{}
	clsiOptions.FillFromEnv()
	clsiManager, err := clsi.New(&clsiOptions)
	if err != nil {
		panic(errors.Tag(err, "clsi setup"))
	}

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

	spellingOptions := spellingTypes.Options{}
	spellingOptions.FillFromEnv()
	sm, err := spelling.New(&spellingOptions)
	if err != nil {
		panic(errors.Tag(err, "spelling setup"))
	}

	webOptions := webTypes.Options{}
	webOptions.FillFromEnv()
	h, webManager, err := web.New(
		&webOptions, db, rClient, localURL, dum, clsiManager,
	)
	if err != nil {
		panic(errors.Tag(err, "web setup"))
	}

	co := corsOptions.Parse()
	r := httpUtils.NewRouter(&httpUtils.RouterOptions{
		Ready: func() bool {
			return !rtm.IsShuttingDown()
		},
	})
	realTimeRouter.Add(r, rtm, realTimeOptions.JWT.Project)
	spellingRouter.Add(r, sm, co)
	webRouter.Add(r, webManager, co)
	if h != nil {
		r.Group("/assets").NewRoute().Methods(http.MethodGet).Handler(h)
	}

	eg, ctx := errgroup.WithContext(triggerExitCtx)
	processDocumentUpdatesCtx, stopProcessingDocumentUpdates := context.WithCancel(context.Background())
	eg.Go(func() error {
		webManager.Cron(ctx, false, time.Minute)
		return nil
	})
	eg.Go(func() error {
		clsiManager.PeriodicCleanup(ctx)
		return nil
	})
	eg.Go(func() error {
		dum.ProcessDocumentUpdates(processDocumentUpdatesCtx)
		return nil
	})
	eg.Go(func() error {
		rtm.PeriodicCleanup(ctx)
		return nil
	})

	server := http.Server{
		Addr:    addr,
		Handler: r,
	}
	eg.Go(func() error {
		return server.ListenAndServe()
	})
	eg.Go(func() error {
		<-ctx.Done()
		// Shutdown sequence:
		// - Stop accepting new websocket connections
		rtm.InitiateGracefulShutdown()
		// - Stop accepting new HTTP requests
		ctx2, done := context.WithTimeout(context.Background(), 15*time.Second)
		defer done()
		pendingShutdown := pendingOperation.TrackOperation(func() error {
			return server.Shutdown(ctx2)
		})
		// - Ask clients to tear down websocket connections
		rtm.TriggerGracefulReconnect()
		// - Wait for existing HTTP requests to finish processing
		err2 := pendingShutdown.Wait(ctx2)
		// - Close remaining websockets
		rtm.DisconnectAll()
		// - Stop processing of document-updates -- keep going until after all
		//    editor sessions had time to flush ahead of disconnecting their
		//    websocket connection.
		stopProcessingDocumentUpdates()
		return err2
	})
	if err = eg.Wait(); err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}
