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

	"golang.org/x/sync/errgroup"

	"github.com/das7pad/overleaf-go/cmd/pkg/utils"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/httpUtils"
	"github.com/das7pad/overleaf-go/pkg/options/listenAddress"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater"
	documentUpdaterTypes "github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
)

func main() {
	triggerExitCtx, triggerExit := signal.NotifyContext(
		context.Background(), syscall.SIGINT, syscall.SIGUSR1, syscall.SIGTERM,
	)
	defer triggerExit()

	rClient := utils.MustConnectRedis(triggerExitCtx)
	db := utils.MustConnectPostgres(triggerExitCtx)

	dumOptions := documentUpdaterTypes.Options{}
	dumOptions.FillFromEnv("DOCUMENT_UPDATER_OPTIONS")
	dum, err := documentUpdater.New(&dumOptions, db, rClient)
	if err != nil {
		panic(errors.Tag(err, "document-updater setup"))
	}

	server := http.Server{
		Addr:    listenAddress.Parse(3003),
		Handler: httpUtils.NewRouter(&httpUtils.RouterOptions{}),
	}
	eg, ctx := errgroup.WithContext(triggerExitCtx)
	eg.Go(func() error {
		dum.ProcessDocumentUpdates(ctx)
		return nil
	})
	eg.Go(func() error {
		return server.ListenAndServe()
	})
	eg.Go(func() error {
		<-ctx.Done()
		return server.Shutdown(context.Background())
	})
	err = eg.Wait()
	if err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}
