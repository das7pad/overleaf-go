// Golang port of Overleaf
// Copyright (C) 2024 Jakob Ackermann <das7pad@outlook.com>
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
	"os/signal"
	"syscall"

	"github.com/das7pad/overleaf-go/cmd/back-fill-project-last-updated/pkg/backFillProjectLastUpdated"
	"github.com/das7pad/overleaf-go/cmd/pkg/utils"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/services/document-updater/pkg/managers/documentUpdater"
	documentUpdaterTypes "github.com/das7pad/overleaf-go/services/document-updater/pkg/types"
)

func main() {
	ctx, triggerExit := signal.NotifyContext(
		context.Background(), syscall.SIGINT, syscall.SIGTERM,
	)
	defer triggerExit()

	rClient := utils.MustConnectRedis(ctx)
	db := utils.MustConnectPostgres(ctx)

	o := documentUpdaterTypes.Options{}
	o.FillFromEnv()
	dum, err := documentUpdater.New(&o, db, rClient)
	if err != nil {
		panic(err)
	}
	ok, err := dum.FlushAll(ctx)
	if !ok && err == nil {
		err = errors.New("soft failure")
	}
	if err != nil {
		panic(errors.Tag(err, "flush failed"))
	}
	if err = backFillProjectLastUpdated.Run(ctx, db); err != nil {
		panic(err)
	}
	log.Println("Done.")
}
