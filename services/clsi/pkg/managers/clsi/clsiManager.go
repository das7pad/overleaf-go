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

package clsi

import (
	"context"
	"log"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/constants"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/managers/clsi/internal/project"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

type Manager interface {
	CleanupOldProjects(ctx context.Context, threshold time.Time) error

	ClearCache(
		ctx context.Context,
		projectId sharedTypes.UUID,
		userId sharedTypes.UUID,
	) error

	Compile(
		ctx context.Context,
		projectId sharedTypes.UUID,
		userId sharedTypes.UUID,
		request *types.CompileRequest,
		response *types.CompileResponse,
	) error

	GetCapacity() (int64, error)

	HealthCheck(ctx context.Context) error

	PeriodicCleanup(ctx context.Context)

	StartInBackground(ctx context.Context, projectId, userId sharedTypes.UUID, request *types.StartInBackgroundRequest) error

	StopCompile(
		ctx context.Context,
		projectId sharedTypes.UUID,
		userId sharedTypes.UUID,
	) error

	SyncFromCode(
		ctx context.Context,
		projectId sharedTypes.UUID,
		userId sharedTypes.UUID,
		request *types.SyncFromCodeRequest,
		positions *types.PDFPositions,
	) error

	SyncFromPDF(
		ctx context.Context,
		projectId sharedTypes.UUID,
		userId sharedTypes.UUID,
		request *types.SyncFromPDFRequest,
		positions *types.CodePositions,
	) error

	WordCount(
		ctx context.Context,
		projectId sharedTypes.UUID,
		userId sharedTypes.UUID,
		request *types.WordCountRequest,
		words *types.Words,
	) error
}

func New(options *types.Options) (Manager, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}

	pm, err := project.NewManager(options)
	if err != nil {
		return nil, err
	}

	nCores := float64(runtime.NumCPU())
	maxSystemLoad := nCores
	if nCores > 1 {
		maxSystemLoad -= 0.25
	}

	return &manager{
		options:       options,
		maxSystemLoad: maxSystemLoad,

		healthCheckMux:       sync.Mutex{},
		healthCheckExpiresAt: time.Now(),

		getCapacityMux:       sync.Mutex{},
		getCapacityExpiresAt: time.Now(),

		pm: pm,
	}, nil
}

type manager struct {
	options       *types.Options
	maxSystemLoad float64

	healthCheckMux       sync.Mutex
	healthCheckErr       error
	healthCheckExpiresAt time.Time

	getCapacityMux       sync.Mutex
	getCapacityCapacity  int64
	getCapacityErr       error
	getCapacityExpiresAt time.Time

	pm project.Manager
}

func (m *manager) CleanupOldProjects(ctx context.Context, threshold time.Time) error {
	return m.pm.CleanupOldProjects(ctx, threshold)
}

func (m *manager) ClearCache(ctx context.Context, projectId sharedTypes.UUID, userId sharedTypes.UUID) error {
	return m.pm.CleanupProject(ctx, projectId, userId)
}

func (m *manager) Compile(ctx context.Context, projectId sharedTypes.UUID, userId sharedTypes.UUID, request *types.CompileRequest, response *types.CompileResponse) error {
	response.Timings.CompileE2E.Begin()
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
			err := p.Compile(ctx, request, response)
			response.Timings.CompileE2E.End()
			return err
		},
	)
}

func (m *manager) GetCapacity() (int64, error) {
	m.getCapacityMux.Lock()
	defer m.getCapacityMux.Unlock()
	if m.getCapacityExpiresAt.After(time.Now()) {
		return m.getCapacityCapacity, m.getCapacityErr
	}
	capacity, err := m.refreshGetCapacity()
	m.getCapacityCapacity = capacity
	m.getCapacityErr = err
	m.getCapacityExpiresAt = time.Now().Add(m.options.GetCapacityRefreshEvery)
	return capacity, err
}

const loadBase = 1 << 16

func (m *manager) refreshGetCapacity() (int64, error) {
	var info syscall.Sysinfo_t
	if err := syscall.Sysinfo(&info); err != nil {
		return 0, err
	}
	load1 := float64(info.Loads[0]) / loadBase
	capacity := int64(100 * (m.maxSystemLoad - load1) / m.maxSystemLoad)

	if capacity < 0 {
		capacity = 0
	}
	return capacity, nil
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
	m.healthCheckExpiresAt = time.Now().Add(m.options.HealthCheckRefreshEvery)
	return err
}

func (m *manager) refreshHealthCheck(ctx context.Context) error {
	content := `
\documentclass{article}
\begin{document}
Hello world
\end{document}
`
	req := &types.CompileRequest{
		Options: types.CompileOptions{
			Check:        types.NoCheck,
			CompileGroup: sharedTypes.StandardCompileGroup,
			Compiler:     sharedTypes.PDFLatex,
			ImageName:    m.options.AllowedImages[0],
			SyncType:     types.SyncTypeFull,
			Timeout:      sharedTypes.ComputeTimeout(10 * time.Second),
		},
		Resources: types.Resources{
			&types.Resource{
				Path:    "main.tex",
				Content: &content,
			},
		},
		RootResourcePath: "main.tex",
	}
	resp := &types.CompileResponse{
		OutputFiles: make(types.OutputFiles, 0),
	}
	err := m.Compile(
		ctx,
		sharedTypes.UUID{},
		sharedTypes.UUID{},
		req,
		resp,
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
	for {
		nextCleanup := time.NewTimer(m.options.ProjectCacheDuration / 2)
		err := m.CleanupOldProjects(
			ctx,
			time.Now().Add(-m.options.ProjectCacheDuration),
		)
		if err != nil && err != ctx.Err() {
			log.Printf("cleanup failed: %s", err)
		}
		select {
		case <-ctx.Done():
			nextCleanup.Stop()
			return
		case <-nextCleanup.C:
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

func (m *manager) StopCompile(ctx context.Context, projectId sharedTypes.UUID, userId sharedTypes.UUID) error {
	p, err := m.pm.GetProject(ctx, projectId, userId)
	if err != nil {
		return err
	}
	return p.StopCompile(ctx)
}

func (m *manager) SyncFromCode(ctx context.Context, projectId sharedTypes.UUID, userId sharedTypes.UUID, request *types.SyncFromCodeRequest, positions *types.PDFPositions) error {
	if err := request.Validate(); err != nil {
		return err
	}

	return m.operateOnProjectWithRecovery(
		ctx,
		projectId,
		userId,
		func(p project.Project) error {
			return p.SyncFromCode(ctx, request, positions)
		},
	)
}

func (m *manager) SyncFromPDF(ctx context.Context, projectId sharedTypes.UUID, userId sharedTypes.UUID, request *types.SyncFromPDFRequest, positions *types.CodePositions) error {
	if err := request.Validate(); err != nil {
		return err
	}

	return m.operateOnProjectWithRecovery(
		ctx,
		projectId,
		userId,
		func(p project.Project) error {
			return p.SyncFromPDF(ctx, request, positions)
		},
	)
}

func (m *manager) WordCount(ctx context.Context, projectId sharedTypes.UUID, userId sharedTypes.UUID, request *types.WordCountRequest, words *types.Words) error {
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
			return p.WordCount(ctx, request, words)
		},
	)
}

func (m *manager) operateOnProjectWithRecovery(ctx context.Context, projectId sharedTypes.UUID, userId sharedTypes.UUID, fn func(p project.Project) error) error {
	var lastErr error
	for i := 0; i < 3; i++ {
		p, err := m.pm.GetProject(ctx, projectId, userId)
		if err != nil {
			if err == project.IsDeadError {
				lastErr = err
				continue
			}
			return err
		}
		err = fn(p)
		if err != nil {
			if err == project.IsDeadError {
				lastErr = err
				continue
			}
			return err
		}
		return nil
	}
	return lastErr
}
