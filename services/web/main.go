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

	"github.com/das7pad/overleaf-go/cmd/pkg/utils"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/options/corsOptions"
	"github.com/das7pad/overleaf-go/pkg/options/env"
	"github.com/das7pad/overleaf-go/pkg/options/listenAddress"
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

	webOptions := webTypes.Options{}
	webOptions.FillFromEnv("WEB_OPTIONS")
	webManager, err := web.New(&webOptions, db, rClient, localUrl, nil)
	if err != nil {
		panic(errors.Tag(err, "web setup"))
	}

	go func() {
		time.Sleep(time.Duration(rand.Int63n(int64(time.Hour))))
		if !webManager.CronOnce(triggerExitCtx, false) {
			log.Println("cron failed")
		}
	}()

	if len(os.Args) > 0 && os.Args[len(os.Args)-1] == "cron" {
		if !webManager.CronOnce(triggerExitCtx, env.GetBool("DRY_RUN_CRON")) {
			os.Exit(42)
		} else {
			os.Exit(0)
		}
		return
	}

	r := router.New(webManager, corsOptions.Parse())
	if err = http.ListenAndServe(addr, r); err != nil {
		panic(err)
	}
}
