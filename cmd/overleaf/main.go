// Golang port of Overleaf
// Copyright (C) 2022 Jakob Ackermann <das7pad@outlook.com>
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

	"github.com/das7pad/overleaf-go/cmd/internal/utils"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/httpUtils"
	envUtils "github.com/das7pad/overleaf-go/pkg/options/utils"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi"
	clsiTypes "github.com/das7pad/overleaf-go/services/clsi/pkg/types"
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
		context.Background(), syscall.SIGINT, syscall.SIGUSR1, syscall.SIGTERM,
	)
	defer triggerExit()

	rClient := utils.MustConnectRedis(10 * time.Second)
	db := utils.MustConnectPostgres(10 * time.Second)
	addr := "127.0.0.1:3000"
	localUrl := "http://" + addr

	clsiOptions := clsiTypes.Options{}
	envUtils.ParseJSONFromEnv("CLSI_OPTIONS", &clsiOptions)
	clsiManager, err := clsi.New(&clsiOptions)
	if err != nil {
		panic(errors.Tag(err, "clsi setup"))
	}

	realTimeOptions := realTimeTypes.Options{}
	envUtils.ParseJSONFromEnv("REAL_TIME_OPTIONS", &realTimeOptions)
	rtm, err := realTime.New(context.Background(), &realTimeOptions, db, rClient)
	if err != nil {
		panic(errors.Tag(err, "realTime setup"))
	}

	spellingOptions := spellingTypes.Options{}
	envUtils.ParseJSONFromEnv("SPELLING_OPTIONS", &spellingOptions)
	sm, err := spelling.New(&spellingOptions)
	if err != nil {
		panic(errors.Tag(err, "spelling setup"))
	}

	webOptions := webTypes.Options{}
	envUtils.ParseJSONFromEnv("WEB_OPTIONS", &webOptions)
	webManager, err := web.New(&webOptions, db, rClient, localUrl, clsiManager)
	if err != nil {
		panic(errors.Tag(err, "web setup"))
	}

	corsOptions := httpUtils.CORSOptions{}

	r := webRouter.New(webManager, corsOptions)
	realTimeRouter.Add(r, rtm, webOptions.JWT.RealTime)
	spellingRouter.Add(r, sm)

	t := time.NewTicker(15 * time.Minute)
	go func() {
		for range t.C {
			webManager.Cron(triggerExitCtx, false)
		}
	}()
	clsiManager.PeriodicCleanup(triggerExitCtx)

	server := http.Server{
		Addr:    addr,
		Handler: r,
	}
	var errServe error
	go func() {
		errServe = server.ListenAndServe()
		triggerExit()
	}()

	<-triggerExitCtx.Done()
	t.Stop()
	rtm.GracefulShutdown()
	errClose := server.Close()
	if errServe != nil && errServe != http.ErrServerClosed {
		panic(errServe)
	}
	if errClose != nil {
		panic(errClose)
	}
}
