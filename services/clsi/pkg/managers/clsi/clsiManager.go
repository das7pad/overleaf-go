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

package clsi

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/constants"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi/internal/project"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

type Manager interface {
	ClearCache(projectId sharedTypes.UUID, userId sharedTypes.UUID) error
	Compile(ctx context.Context, projectId sharedTypes.UUID, userId sharedTypes.UUID, request *types.CompileRequest, response *types.CompileResponse) error
	HealthCheck(ctx context.Context) error
	PeriodicCleanup(ctx context.Context)
	StartInBackground(ctx context.Context, projectId, userId sharedTypes.UUID, request *types.StartInBackgroundRequest) error
	SyncFromCode(ctx context.Context, projectId sharedTypes.UUID, userId sharedTypes.UUID, request *types.SyncFromCodeRequest, response *types.SyncFromCodeResponse) error
	SyncFromPDF(ctx context.Context, projectId sharedTypes.UUID, userId sharedTypes.UUID, request *types.SyncFromPDFRequest, response *types.SyncFromPDFResponse) error
	WordCount(ctx context.Context, projectId sharedTypes.UUID, userId sharedTypes.UUID, request *types.WordCountRequest, response *types.WordCountResponse) error
}

func New(options *types.Options) (Manager, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}

	pm, err := project.NewManager(options)
	if err != nil {
		return nil, err
	}

	return &manager{
		refreshHealthCheckEvery: options.RefreshHealthCheckEvery,
		projectCacheDuration:    options.ProjectCacheDuration,
		projectRunnerMaxAge:     options.ProjectRunnerMaxAge,
		healthCheckImageName:    options.AllowedImages[0],

		healthCheckMux:       sync.Mutex{},
		healthCheckExpiresAt: time.Now(),

		pm: pm,
	}, nil
}

type manager struct {
	refreshHealthCheckEvery time.Duration
	projectCacheDuration    time.Duration
	projectRunnerMaxAge     time.Duration
	healthCheckImageName    sharedTypes.ImageName

	healthCheckMux       sync.Mutex
	healthCheckErr       error
	healthCheckExpiresAt time.Time

	pm project.Manager
}

func (m *manager) ClearCache(projectId sharedTypes.UUID, userId sharedTypes.UUID) error {
	return m.pm.ClearProjectCache(projectId, userId)
}

func (m *manager) Compile(ctx context.Context, projectId sharedTypes.UUID, userId sharedTypes.UUID, request *types.CompileRequest, response *types.CompileResponse) error {
	response.Timings.CompileE2E.Begin()
	defer response.Timings.CompileE2E.End()
	if err := request.Preprocess(); err != nil {
		return err
	}
	if err := request.Validate(); err != nil {
		return err
	}

	return m.operateOnProjectWithRecovery(
		ctx,
		projectId,
		userId,
		func(p project.Project) error {
			return p.Compile(ctx, request, response)
		},
	)
}

func (m *manager) HealthCheck(ctx context.Context) error {
	m.healthCheckMux.Lock()
	defer m.healthCheckMux.Unlock()
	if m.healthCheckExpiresAt.After(time.Now()) {
		return m.healthCheckErr
	}
	err := m.refreshHealthCheck(ctx)
	if err != nil && err == ctx.Err() {
		// Do not persist a request cancelled/timed out error.
		return err
	}
	m.healthCheckErr = err
	m.healthCheckExpiresAt = time.Now().Add(m.refreshHealthCheckEvery)
	return err
}

func (m *manager) refreshHealthCheck(ctx context.Context) error {
	content := `
\documentclass{article}
\begin{document}
Hello world
\end{document}
`
	req := types.CompileRequest{
		Options: types.CompileOptions{
			Check:            types.NoCheck,
			CompileGroup:     sharedTypes.StandardCompileGroup,
			Compiler:         sharedTypes.PDFLaTeX,
			ImageName:        m.healthCheckImageName,
			RootResourcePath: "main.tex",
			SyncState:        "42",
			SyncType:         types.SyncTypeFullIncremental,
			Timeout:          sharedTypes.ComputeTimeout(10 * time.Second),
		},
		Resources: types.Resources{
			&types.Resource{
				Path:    "main.tex",
				Content: content,
			},
		},
	}
	resp := types.CompileResponse{
		OutputFiles: make(types.OutputFiles, 0),
	}
	err := m.Compile(
		ctx,
		sharedTypes.UUID{},
		sharedTypes.UUID{},
		&req,
		&resp,
	)
	if err != nil {
		return err
	}
	if resp.Status != constants.Success {
		return errors.New("non success")
	}
	foundLog := false
	foundPDF := false
	for _, outputFile := range resp.OutputFiles {
		if outputFile.Type == "log" {
			foundLog = true
		}
		if outputFile.Type == "pdf" {
			foundPDF = true
		}
		if foundPDF && foundLog {
			break
		}
	}
	if !foundLog {
		return errors.New("missing log")
	}
	if !foundPDF {
		return errors.New("missing pdf")
	}
	return nil
}

func (m *manager) PeriodicCleanup(ctx context.Context) {
	inter := min(m.projectCacheDuration, m.projectRunnerMaxAge)
	nextCleanup := time.NewTicker(inter / 10)
	now := time.Now()
	for {
		{
			err := m.pm.CleanupOldProjects(
				ctx,
				now.Add(-m.projectCacheDuration),
			)
			if err != nil && err != ctx.Err() {
				log.Printf("cleanup old projects failed: %s", err)
			}
		}
		{
			err := m.pm.StopExpiredRunners(
				ctx,
				now.Add(-m.projectRunnerMaxAge),
			)
			if err != nil && err != ctx.Err() {
				log.Printf("stopping expired failed: %s", err)
			}
		}
		select {
		case <-ctx.Done():
			nextCleanup.Stop()
			return
		case now = <-nextCleanup.C:
		}
	}
}

func (m *manager) StartInBackground(ctx context.Context, projectId, userId sharedTypes.UUID, request *types.StartInBackgroundRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}

	return m.operateOnProjectWithRecovery(
		ctx,
		projectId,
		userId,
		func(p project.Project) error {
			p.StartInBackground(request.ImageName)
			return nil
		},
	)
}

func (m *manager) SyncFromCode(ctx context.Context, projectId sharedTypes.UUID, userId sharedTypes.UUID, request *types.SyncFromCodeRequest, response *types.SyncFromCodeResponse) error {
	if err := request.Validate(); err != nil {
		return err
	}

	return m.operateOnProjectWithRecovery(
		ctx,
		projectId,
		userId,
		func(p project.Project) error {
			return p.SyncFromCode(ctx, request, response)
		},
	)
}

func (m *manager) SyncFromPDF(ctx context.Context, projectId sharedTypes.UUID, userId sharedTypes.UUID, request *types.SyncFromPDFRequest, response *types.SyncFromPDFResponse) error {
	if err := request.Validate(); err != nil {
		return err
	}

	return m.operateOnProjectWithRecovery(
		ctx,
		projectId,
		userId,
		func(p project.Project) error {
			return p.SyncFromPDF(ctx, request, response)
		},
	)
}

func (m *manager) WordCount(ctx context.Context, projectId sharedTypes.UUID, userId sharedTypes.UUID, request *types.WordCountRequest, response *types.WordCountResponse) error {
	if err := request.Preprocess(); err != nil {
		return err
	}
	if err := request.Validate(); err != nil {
		return err
	}

	return m.operateOnProjectWithRecovery(
		ctx,
		projectId,
		userId,
		func(p project.Project) error {
			return p.WordCount(ctx, request, response)
		},
	)
}

func (m *manager) operateOnProjectWithRecovery(ctx context.Context, projectId sharedTypes.UUID, userId sharedTypes.UUID, fn func(p project.Project) error) error {
	for i := 0; i < 3; i++ {
		p, err := m.pm.GetProject(ctx, projectId, userId)
		if err != nil {
			if err == project.ErrIsDead {
				continue
			}
			return err
		}
		if err = fn(p); err != nil {
			if err == project.ErrIsDead {
				continue
			}
			return err
		}
		return nil
	}
	return project.ErrIsDead
}
