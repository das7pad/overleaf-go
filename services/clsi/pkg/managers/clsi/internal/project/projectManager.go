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

package project

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/edgedb/edgedb-go"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi/internal/commandRunner"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi/internal/draftMode"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi/internal/latexRunner"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi/internal/outputCache"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi/internal/outputFileFinder"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi/internal/resourceWriter"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi/internal/rootDocAlias"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi/internal/syncTex"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi/internal/wordCounter"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

type Manager interface {
	GetProject(
		ctx context.Context,
		projectId edgedb.UUID,
		userId edgedb.UUID,
	) (Project, error)

	CleanupProject(
		ctx context.Context,
		projectId edgedb.UUID,
		userId edgedb.UUID,
	) error

	CleanupOldProjects(
		ctx context.Context,
		activeThreshold time.Time,
	) error
}

type managers struct {
	draftMode    draftMode.Manager
	outputCache  outputCache.Manager
	rootDocAlias rootDocAlias.Manager
	runner       commandRunner.Runner
	syncTex      syncTex.Manager
	wordCounter  wordCounter.Counter
	writer       resourceWriter.ResourceWriter
	latexRunner  latexRunner.LatexRunner
}

func NewManager(options *types.Options) (Manager, error) {
	createPaths := []string{
		string(options.OutputBaseDir),
		string(options.CompileBaseDir),
	}
	for _, path := range createPaths {
		if err := os.MkdirAll(path, 0755); err != nil {
			return nil, err
		}
	}

	finder := outputFileFinder.New(options)

	writer, err := resourceWriter.New(options, finder)
	if err != nil {
		return nil, err
	}

	runner, err := commandRunner.New(options)
	if err != nil {
		return nil, err
	}

	return &manager{
		l:        sync.RWMutex{},
		projects: make(projectsMap, 0),

		managers: &managers{
			draftMode:    draftMode.New(),
			latexRunner:  latexRunner.New(options),
			outputCache:  outputCache.New(options, finder),
			rootDocAlias: rootDocAlias.New(),
			runner:       runner,
			syncTex:      syncTex.New(options, runner),
			wordCounter:  wordCounter.New(options),
			writer:       writer,
		},

		options: options,
	}, nil
}

type projectKey struct {
	ProjectId edgedb.UUID
	UserId    edgedb.UUID
}

func (k projectKey) String() string {
	return k.ProjectId.String() + "-" + k.UserId.String()
}

type projectsMap map[projectKey]Project

type manager struct {
	l        sync.RWMutex
	projects projectsMap

	options *types.Options

	*managers
}

func (m *manager) getUnhealthyProjects(activeThreshold time.Time) projectsMap {
	m.l.RLock()
	defer m.l.RUnlock()
	unhealthyProjects := make(projectsMap, 0)
	for key, p := range m.projects {
		if !p.IsHealthy(activeThreshold) {
			unhealthyProjects[key] = p
		}
	}
	return unhealthyProjects
}

func (m *manager) CleanupOldProjects(ctx context.Context, activeThreshold time.Time) error {
	// map iteration is randomized in Golang, so broken projects should not
	//  let the project cleanup get stuck.
	// Operate on a shadow copy as we would need to lock map access otherwise.
	for key, p := range m.getUnhealthyProjects(activeThreshold) {
		// Trigger cleanup on instance.
		if err := p.CleanupUnlessHealthy(ctx, activeThreshold); err != nil {
			return errors.Tag(err, "cleanup failed for "+key.String())
		}
		if p.IsDead() {
			// Serialize map access shortly when cleanup was actioned only.
			m.cleanupIfStillDead(key, p)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	return nil
}

func (m *manager) CleanupProject(ctx context.Context, projectId edgedb.UUID, userId edgedb.UUID) error {
	key := projectKey{
		ProjectId: projectId,
		UserId:    userId,
	}

	p := m.getExistingProject(key)
	if p == nil {
		// Already cleaned up.
		return nil
	}

	if err := p.Cleanup().Wait(ctx); err != nil {
		return err
	}

	m.cleanupIfStillDead(key, p)
	return nil
}

func (m *manager) GetProject(ctx context.Context, projectId edgedb.UUID, userId edgedb.UUID) (Project, error) {
	p, err := m.getOrCreateProject(ctx, projectId, userId)
	if err != nil {
		return nil, err
	}
	p.Touch()
	return p, nil
}

func (m *manager) getOrCreateProject(ctx context.Context, projectId edgedb.UUID, userId edgedb.UUID) (Project, error) {
	key := projectKey{
		ProjectId: projectId,
		UserId:    userId,
	}

	p := m.getExistingProject(key)
	if p != nil {
		if err := ctx.Err(); err != nil {
			// Cancelled or timed out.
			return nil, err
		}
		if !p.IsDead() {
			// Return alive project
			return p, nil
		} else {
			if err := p.Cleanup().Wait(ctx); err != nil {
				// Stuck in cleanup :/
				return nil, err
			}
			// Replace the dead instance with a new one.
		}
	}
	deadProject := p

	newP, err := newProject(projectId, userId, m.managers, m.options)
	if err != nil {
		return nil, err
	}

	p = m.setIfStillDead(key, deadProject, newP)
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return p, nil
}

func (m *manager) setIfStillDead(key projectKey, deadProject Project, newProject Project) Project {
	m.l.Lock()
	defer m.l.Unlock()

	p, exists := m.projects[key]
	if exists && p != deadProject {
		// Someone else won the race, return their Project instance.
		return p
	}
	m.projects[key] = newProject
	return newProject
}

func (m *manager) cleanupIfStillDead(key projectKey, deadProject Project) {
	m.l.Lock()
	defer m.l.Unlock()

	p := m.projects[key]
	if p == deadProject {
		// Nobody replaced the project yet.
		delete(m.projects, key)
	}
}

func (m *manager) getExistingProject(key projectKey) Project {
	m.l.RLock()
	defer m.l.RUnlock()

	return m.projects[key]
}
