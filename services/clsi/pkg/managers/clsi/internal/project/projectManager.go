// Golang port of Overleaf
// Copyright (C) 2021-2024 Jakob Ackermann <das7pad@outlook.com>
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

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi/internal/commandRunner"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi/internal/draftMode"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi/internal/latexRunner"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi/internal/outputCache"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi/internal/outputFileFinder"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi/internal/pdfCaching"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi/internal/resourceWriter"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi/internal/rootDocAlias"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi/internal/syncTex"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi/internal/wordCounter"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

type Manager interface {
	GetProject(ctx context.Context, projectId sharedTypes.UUID, userId sharedTypes.UUID) (Project, error)
	CleanupProject(projectId sharedTypes.UUID, userId sharedTypes.UUID) error
	ClearProjectCache(projectId, userId sharedTypes.UUID) error
	CleanupOldProjects(ctx context.Context, activeThreshold time.Time) error
	StopExpiredRunners(ctx context.Context, activeThreshold time.Time) error
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
	pdfCaching   pdfCaching.Manager
}

func NewManager(options *types.Options) (Manager, error) {
	createPaths := []string{
		string(options.OutputBaseDir),
		string(options.CompileBaseDir),
	}
	for _, path := range createPaths {
		if err := os.MkdirAll(path, 0o755); err != nil {
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
			pdfCaching:   pdfCaching.New(options),
		},

		paths: options.Paths,
	}, nil
}

type projectKey struct {
	ProjectId sharedTypes.UUID
	UserId    sharedTypes.UUID
}

func (k projectKey) String() string {
	return k.ProjectId.Concat('-', k.UserId)
}

type projectsMap map[projectKey]Project

type manager struct {
	l        sync.RWMutex
	projects projectsMap

	paths types.Paths

	*managers
}

func (m *manager) getUnhealthyProjects(activeThreshold time.Time) projectsMap {
	m.l.RLock()
	n := 1 + len(m.projects)/10 // look at 10% of the projects per iteration
	m.l.RUnlock()               // release lock for map allocation
	unhealthyProjects := make(projectsMap, n)
	m.l.RLock()
	defer m.l.RUnlock()
	for key, p := range m.projects {
		if !p.IsHealthy(activeThreshold) {
			unhealthyProjects[key] = p
		}
		if n--; n <= 0 {
			break
		}
	}
	return unhealthyProjects
}

func (m *manager) getProjectsWithExpiredSetup(now time.Time) projectsMap {
	m.l.RLock()
	n := 1 + len(m.projects)/10 // look at 10% of the projects per iteration
	m.l.RUnlock()               // release lock for map allocation
	expired := make(projectsMap, n)
	m.l.RLock()
	defer m.l.RUnlock()
	for key, p := range m.projects {
		if p.RunnerExpired(now) {
			expired[key] = p
		}
		if n--; n <= 0 {
			break
		}
	}
	return expired
}

func (m *manager) CleanupOldProjects(ctx context.Context, activeThreshold time.Time) error {
	// map iteration is randomized in Golang, so broken projects should not
	//  let the project cleanup get stuck.
	// Operate on a shadow copy as we would need to lock map access otherwise.
	for key, p := range m.getUnhealthyProjects(activeThreshold) {
		// Trigger cleanup on instance.
		if err := p.CleanupUnlessHealthy(activeThreshold); err != nil {
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

func (m *manager) StopExpiredRunners(ctx context.Context, activeThreshold time.Time) error {
	// map iteration is randomized in Golang, so broken projects should not
	//  let the project cleanup get stuck.
	// Operate on a shadow copy as we would need to lock map access otherwise.
	for key, p := range m.getProjectsWithExpiredSetup(activeThreshold) {
		if err := p.StopExpiredRunner(activeThreshold); err != nil {
			return errors.Tag(err, "cleanup failed for "+key.String())
		}
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	return nil
}

func (m *manager) ClearProjectCache(projectId, userId sharedTypes.UUID) error {
	key := projectKey{
		ProjectId: projectId,
		UserId:    userId,
	}
	if p, exists := m.getExistingProject(key); exists {
		return p.ClearCache()
	}
	return nil
}

func (m *manager) CleanupProject(projectId sharedTypes.UUID, userId sharedTypes.UUID) error {
	key := projectKey{
		ProjectId: projectId,
		UserId:    userId,
	}

	if p, exists := m.getExistingProject(key); exists {
		if err := p.Cleanup(); err != nil {
			return err
		}
		m.cleanupIfStillDead(key, p)
	}
	return nil
}

func (m *manager) GetProject(ctx context.Context, projectId sharedTypes.UUID, userId sharedTypes.UUID) (Project, error) {
	p, err := m.getOrCreateProject(ctx, projectId, userId)
	if err != nil {
		return nil, err
	}
	p.Touch()
	return p, nil
}

func (m *manager) getOrCreateProject(ctx context.Context, projectId sharedTypes.UUID, userId sharedTypes.UUID) (Project, error) {
	key := projectKey{
		ProjectId: projectId,
		UserId:    userId,
	}

	p, exists := m.getExistingProject(key)
	if exists {
		if err := ctx.Err(); err != nil {
			// Cancelled or timed out.
			return nil, err
		}
		if !p.IsDead() {
			// Return alive project
			return p, nil
		}
		if err := p.Cleanup(); err != nil {
			// Stuck in cleanup :/
			return nil, err
		}
		// Replace the dead instance with a new one.
	}
	deadProject := p

	newP, err := newProject(projectId, userId, m.managers, m.paths)
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

	if p, exists := m.projects[key]; exists && p != deadProject {
		// Someone else won the race, return their Project instance.
		return p
	}
	m.projects[key] = newProject
	return newProject
}

func (m *manager) cleanupIfStillDead(key projectKey, deadProject Project) {
	m.l.Lock()
	defer m.l.Unlock()

	if p, exists := m.projects[key]; exists && p == deadProject {
		// Nobody replaced the project yet.
		delete(m.projects, key)
	}
}

func (m *manager) getExistingProject(key projectKey) (Project, bool) {
	m.l.RLock()
	defer m.l.RUnlock()

	p, exists := m.projects[key]
	return p, exists
}
