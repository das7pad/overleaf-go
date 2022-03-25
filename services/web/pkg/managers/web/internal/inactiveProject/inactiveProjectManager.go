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

package inactiveProject

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/models/project"
	"github.com/das7pad/overleaf-go/services/docstore/pkg/managers/docstore"
	"github.com/das7pad/overleaf-go/services/web/pkg/types"
)

type Manager interface {
	ArchiveInactiveProjects(ctx context.Context, dryRun bool) error
}

func New(options *types.Options, pm project.Manager, dm docstore.Manager) Manager {
	return &manager{
		age: options.ProjectsInactiveAfter,
		dm:  dm,
		pm:  pm,
	}
}

type manager struct {
	age time.Duration
	dm  docstore.Manager
	pm  project.Manager
}

func (m *manager) ArchiveInactiveProjects(ctx context.Context, dryRun bool) error {
	nFailed := 0
	// NOTE: Docstore archives docs in parallel, process projects in sequence.
	err := m.pm.ProcessInactiveProjects(ctx, m.age, func(projectId edgedb.UUID) bool {
		if dryRun {
			log.Printf("dry-run archiving inactive project %s", projectId)
			return false
		}
		if err := m.ArchiveProject(ctx, projectId); err != nil {
			err = errors.Tag(
				err, "archiving failed for old project "+projectId.String(),
			)
			log.Println(err.Error())
			nFailed += 1
		}
		return nFailed == 0
	})
	if err != nil {
		err = errors.Tag(err, "query projects")
	}
	if nFailed != 0 {
		err = errors.Merge(err, errors.New(fmt.Sprintf(
			"archiving failed for %d projects", nFailed,
		)))
	}
	return err
}

func (m *manager) ArchiveProject(ctx context.Context, projectId edgedb.UUID) error {
	if err := m.dm.ArchiveProject(ctx, projectId); err != nil {
		return err
	}
	if err := m.pm.MarkAsInActive(ctx, projectId); err != nil {
		return err
	}
	return nil
}
