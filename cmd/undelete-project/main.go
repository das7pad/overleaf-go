// Golang port of Overleaf
// Copyright (C) 2023 Jakob Ackermann <das7pad@outlook.com>
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
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/das7pad/overleaf-go/cmd/pkg/utils"
	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

func main() {
	ctx, done := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	defer done()

	var projectIdRaw string
	flag.StringVar(&projectIdRaw, "project-id", sharedTypes.AllZeroUUID, "project id")
	var ownerUserIdRaw string
	flag.StringVar(&ownerUserIdRaw, "project-owner-user-id", sharedTypes.AllZeroUUID, "project owner user id")

	flag.Parse()
	projectId, err := sharedTypes.ParseUUID(projectIdRaw)
	if err != nil {
		err = errors.Tag(err, "invalid project-id")
		_, _ = fmt.Fprintf(os.Stderr, "ERR: %s\n", err.Error())
		flag.Usage()
		os.Exit(1)
	}
	userId, err := sharedTypes.ParseUUID(ownerUserIdRaw)
	if err != nil {
		err = errors.Tag(err, "invalid project-owner-user-id")
		_, _ = fmt.Fprintf(os.Stderr, "ERR: %s\n", err.Error())
		flag.Usage()
		os.Exit(1)
	}

	db := utils.MustConnectPostgres(ctx)
	pm := project.New(db)

	name, err := pm.GetDeletedProjectsName(ctx, projectId, userId)
	if err != nil {
		panic(errors.Tag(err, "get deleted projects name"))
	}
	names, err := pm.GetProjectNames(ctx, userId)
	if err != nil {
		panic(errors.Tag(err, "get other project names"))
	}
	name = names.MakeUnique(name)

	if err = pm.Restore(ctx, projectId, userId, name); err != nil {
		panic(errors.Tag(err, "restore project"))
	}
	log.Println("Restored project.")
}
