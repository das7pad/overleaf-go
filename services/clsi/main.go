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

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/options/env"
	"github.com/das7pad/overleaf-go/pkg/options/listenAddress"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi"
	clsiTypes "github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

func main() {
	clsiOptions := clsiTypes.Options{}
	clsiOptions.FillFromEnv("CLSI_OPTIONS")
	clsiManager, err := clsi.New(&clsiOptions)
	if err != nil {
		panic(errors.Tag(err, "clsi setup"))
	}

	backgroundTaskCtx, shutdownBackgroundTasks := context.WithCancel(
		context.Background(),
	)
	go clsiManager.PeriodicCleanup(backgroundTaskCtx)

	loadAgent, err := startLoadAgent(
		listenAddress.Parse(env.GetInt("LOAD_PORT", 3048)),
		env.GetBool("LOAD_SHEDDING"),
		clsiManager.GetCapacity,
	)
	if err != nil {
		panic(err)
	}

	server := http.Server{
		Addr:    listenAddress.Parse(3013),
		Handler: newHTTPController(clsiManager).GetRouter(),
	}
	err = server.ListenAndServe()
	shutdownBackgroundTasks()
	_ = loadAgent.Close()
	if err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}
