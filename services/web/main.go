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
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/cmd/pkg/utils"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/options/corsOptions"
	"github.com/das7pad/overleaf-go/pkg/options/env"
	"github.com/das7pad/overleaf-go/pkg/options/listenAddress"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater"
	documentUpdaterTypes "github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
	"github.com/das7pad/overleaf-go/services/web/pkg/managers/web"
	"github.com/das7pad/overleaf-go/services/web/pkg/router"
	webTypes "github.com/das7pad/overleaf-go/services/web/pkg/types"
)

func main() {
	triggerExitCtx, triggerExit := signal.NotifyContext(
		context.Background(), syscall.SIGINT, syscall.SIGUSR1, syscall.SIGTERM,
	)
	defer triggerExit()

	rand.Seed(time.Now().UnixNano())

	rClient := utils.MustConnectRedis(triggerExitCtx)
	db := utils.MustConnectPostgres(triggerExitCtx)

	addr := listenAddress.Parse(3000)
	localUrl := "http://" + addr
	if strings.HasPrefix(addr, ":") || strings.HasPrefix(addr, "0.0.0.0") {
		localUrl = "http://127.0.0.1" + strings.TrimPrefix(addr, "0.0.0.0")
	}

	dumOptions := documentUpdaterTypes.Options{}
	dumOptions.FillFromEnv()
	dum, err := documentUpdater.New(&dumOptions, db, rClient)
	if err != nil {
		panic(errors.Tag(err, "document-updater setup"))
	}

	webOptions := webTypes.Options{}
	webOptions.FillFromEnv()
	webManager, err := web.New(&webOptions, db, rClient, localUrl, dum, nil)
	if err != nil {
		panic(errors.Tag(err, "web setup"))
	}

	if len(os.Args) > 0 && os.Args[len(os.Args)-1] == "cron" {
		if !webManager.CronOnce(triggerExitCtx, env.GetBool("DRY_RUN_CRON")) {
			os.Exit(42)
		} else {
			os.Exit(0)
		}
		return
	}

	eg, ctx := errgroup.WithContext(triggerExitCtx)
	eg.Go(func() error {
		time.Sleep(time.Duration(rand.Int63n(int64(time.Hour))))
		if !webManager.CronOnce(ctx, false) {
			log.Println("cron failed")
		}
		return nil
	})

	server := http.Server{
		Addr:    addr,
		Handler: router.New(webManager, corsOptions.Parse()),
	}
	eg.Go(func() error {
		return server.ListenAndServe()
	})
	eg.Go(func() error {
		<-ctx.Done()
		waitForSlowRequests, done := context.WithTimeout(
			context.Background(), time.Second*30,
		)
		defer done()
		return server.Shutdown(waitForSlowRequests)
	})
	err = eg.Wait()
	if err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}
