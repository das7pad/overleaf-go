// Golang port of Overleaf
// Copyright (C) 2021 Jakob Ackermann <das7pad@outlook.com>
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
	"os"

	"github.com/das7pad/overleaf-go/services/clsi/pkg/copyFile"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi"
)

func main() {
	o := getOptions()
	backgroundTaskCtx, shutdownBackgroundTasks := context.WithCancel(
		context.Background(),
	)

	if o.copyExecAgent {
		err := copyAgent(o.copyExecAgentSrc, o.copyExecAgentDst)
		if err != nil {
			panic(err)
		}
	}

	cm, err := clsi.New(o.options)
	if err != nil {
		panic(err)
	}
	go cm.PeriodicCleanup(backgroundTaskCtx)

	loadAgent, err := startLoadAgent(o, cm)
	if err != nil {
		panic(err)
	}

	handler := newHttpController(cm)
	server := http.Server{
		Addr:    o.address,
		Handler: handler.GetRouter(),
	}
	err = server.ListenAndServe()
	shutdownBackgroundTasks()
	_ = loadAgent.Close()
	if err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}

func copyAgent(src, dest string) error {
	agent, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		_ = agent.Close()
	}()
	return copyFile.Atomic(agent, dest, true)
}
