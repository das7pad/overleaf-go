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

package project

import (
	"context"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/pendingOperation"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/constants"
	"github.com/das7pad/overleaf-go/services/clsi/pkg/types"
)

type Project interface {
	IsDead() bool
	IsHealthy(activeThreshold time.Time) bool

	Cleanup() error

	CleanupUnlessHealthy(activeThreshold time.Time) error

	ClearCache() error

	Compile(
		ctx context.Context,
		request *types.CompileRequest,
		response *types.CompileResponse,
	) error

	StartInBackground(imageName sharedTypes.ImageName)

	StopCompile(ctx context.Context) error

	SyncFromCode(
		ctx context.Context,
		request *types.SyncFromCodeRequest,
		positions *types.PDFPositions,
	) error

	SyncFromPDF(
		ctx context.Context,
		request *types.SyncFromPDFRequest,
		positions *types.CodePositions,
	) error

	Touch()

	WordCount(
		ctx context.Context,
		request *types.WordCountRequest,
		words *types.Words,
	) error
}

func newProject(
	projectId sharedTypes.UUID,
	userId sharedTypes.UUID,
	m *managers,
	paths types.Paths,
) (Project, error) {
	namespace := types.Namespace(projectId.String() + "-" + userId.String())

	createPaths := []string{
		string(paths.CacheBaseDir.NamespacedCacheDir(namespace)),
		string(paths.CompileBaseDir.CompileDir(namespace)),
		string(paths.OutputBaseDir.OutputDir(namespace)),
		paths.OutputBaseDir.OutputDir(namespace).CompileOutput(),
	}
	for _, path := range createPaths {
		// Any parent directories have been created during manager init.
		if err := os.Mkdir(path, 0o755); err != nil && !os.IsExist(err) {
			return nil, err
		}
	}

	initialState := types.SyncStateCleared
	if state, err := m.writer.GetState(namespace); err == nil {
		initialState = state
	}

	return &project{
		namespace: namespace,
		projectId: projectId,

		state:                 initialState,
		stateMux:              sync.RWMutex{},
		runnerSetupValidUntil: time.Now(),
		runnerSetupMux:        sync.RWMutex{},
		compileMux:            sync.Mutex{},

		managers: m,
	}, nil
}

type project struct {
	dead       atomic.Bool
	lastAccess atomic.Int64
	namespace  types.Namespace
	projectId  sharedTypes.UUID

	state    types.SyncState
	stateMux sync.RWMutex

	runnerSetupValidUntil time.Time
	runnerSetupMux        sync.RWMutex
	pendingRunnerSetup    pendingOperation.WithCancel

	compileMux     sync.Mutex
	pendingCompile pendingOperation.WithCancel

	*managers
}

func (p *project) IsDead() bool {
	return p.dead.Load()
}

func (p *project) IsHealthy(activeThreshold time.Time) bool {
	return !p.IsDead() && p.lastAccess.Load() > activeThreshold.Unix()
}

func (p *project) Cleanup() error {
	p.dead.Store(true)
	return p.triggerCleanup()
}

func (p *project) CleanupUnlessHealthy(activeThreshold time.Time) error {
	if p.IsHealthy(activeThreshold) {
		return nil
	}
	return p.Cleanup()
}

func (p *project) ClearCache() error {
	return p.triggerCleanup()
}

func (p *project) Compile(ctx context.Context, request *types.CompileRequest, response *types.CompileResponse) error {
	options := request.Options
	p.stateMux.RLock()
	defer p.stateMux.RUnlock()

	if err := p.checkIsDead(); err != nil {
		return err
	}
	if err := p.checkSyncState(options.SyncType, options.SyncState); err != nil {
		return err
	}
	pending, err := p.triggerCompile(ctx, request, response)
	if err != nil {
		return err
	}
	return pending.Wait(ctx)
}

func (p *project) triggerCompile(ctx context.Context, request *types.CompileRequest, response *types.CompileResponse) (pendingOperation.WithCancel, error) {
	p.compileMux.Lock()
	defer p.compileMux.Unlock()
	pending := p.pendingCompile
	if pending != nil {
		return nil, &errors.AlreadyCompilingError{}
	}

	pending = pendingOperation.TrackOperationWithCancel(
		ctx,
		func(compileCtx context.Context) error {
			err := p.doCompile(compileCtx, request, response)
			p.compileMux.Lock()
			defer p.compileMux.Unlock()
			if p.pendingCompile == pending {
				p.pendingCompile = nil
			}
			return err
		},
	)
	return pending, nil
}

func (p *project) doCompile(ctx context.Context, request *types.CompileRequest, response *types.CompileResponse) error {
	response.Status = constants.Failure
	response.OutputFiles = make(types.OutputFiles, 0)
	if request.Options.Draft {
		if err := p.draftMode.InjectDraftMode(request.RootDoc); err != nil {
			return err
		}
	}
	p.rootDocAlias.AddAliasDocIfNeeded(request)

	response.Timings.Sync.Begin()
	cache, err := p.writer.SyncResourcesToDisk(
		ctx,
		p.projectId,
		p.namespace,
		request,
	)
	response.Timings.Sync.End()
	if err != nil {
		return err
	}
	p.state = request.Options.SyncState

	err = p.latexRunner.Run(ctx, p.run, p.namespace, request, response)
	if err != nil {
		return err
	}

	response.Timings.Output.Begin()
	outputFiles, hasOutputPDF, err := p.outputCache.SaveOutputFiles(
		ctx,
		cache,
		p.namespace,
	)
	response.Timings.Output.End()
	if response.Status == constants.Success && !hasOutputPDF {
		response.Status = constants.Failure
	}
	if err != nil {
		return err
	}
	response.OutputFiles = outputFiles
	return nil
}

func (p *project) StopCompile(ctx context.Context) error {
	p.compileMux.Lock()
	defer p.compileMux.Unlock()

	pending := p.pendingCompile
	if pending == nil {
		return nil
	}
	pending.Cancel()
	_ = pending.Wait(ctx)
	return nil
}

func (p *project) SyncFromCode(ctx context.Context, request *types.SyncFromCodeRequest, positions *types.PDFPositions) error {
	if err := p.checkStateExpectAnyContent(); err != nil {
		return err
	}

	return p.syncTex.FromCode(
		ctx,
		p.run,
		p.namespace,
		request,
		positions,
	)
}

func (p *project) SyncFromPDF(ctx context.Context, request *types.SyncFromPDFRequest, positions *types.CodePositions) error {
	if err := p.checkStateExpectAnyContent(); err != nil {
		return err
	}

	return p.syncTex.FromPDF(
		ctx,
		p.run,
		p.namespace,
		request,
		positions,
	)
}

func (p *project) Touch() {
	p.lastAccess.Store(time.Now().Unix())
}

func (p *project) WordCount(ctx context.Context, request *types.WordCountRequest, words *types.Words) error {
	p.stateMux.RLock()
	defer p.stateMux.RUnlock()
	if err := p.checkStateExpectAnyContent(); err != nil {
		return err
	}

	return p.wordCounter.Count(
		ctx,
		p.run,
		p.namespace,
		request,
		words,
	)
}

var ErrIsDead = &errors.InvalidStateError{
	Msg: "project is dead",
}

func (p *project) checkIsDead() error {
	if p.IsDead() {
		return ErrIsDead
	}
	return nil
}

func (p *project) checkStateExpectAnyContent() error {
	if err := p.checkIsDead(); err != nil {
		return err
	}
	if p.state == types.SyncStateCleared {
		return &errors.InvalidStateError{
			Msg: "project contents are missing",
		}
	}
	return nil
}

func (p *project) checkSyncState(syncType types.SyncType, state types.SyncState) error {
	if syncType.IsFull() {
		// SyncTypeFull and SyncTypeFullIncremental overwrite everything.
		return nil
	}

	needsFullSync := p.state == "" || p.state == types.SyncStateCleared
	if needsFullSync {
		return &errors.InvalidStateError{
			Msg: "local sync state empty and incoming syncType!=full",
		}
	}

	if p.state != state {
		return &errors.InvalidStateError{
			Msg: "local sync state differs remote state",
		}
	}
	return nil
}

func (p *project) doCleanup() error {
	p.compileMux.Lock()
	if compile := p.pendingCompile; compile != nil {
		compile.Cancel()
	}
	p.compileMux.Unlock()

	p.runnerSetupMux.RLock()
	if setup := p.pendingRunnerSetup; setup != nil {
		setup.Cancel()
	}
	p.runnerSetupMux.RUnlock()

	// Take write lock
	p.stateMux.Lock()
	defer p.stateMux.Unlock()
	p.state = types.SyncStateCleared

	errRunner := p.runner.Stop(p.namespace)
	errWriter := p.writer.Clear(p.projectId, p.namespace)
	errOutputCache := p.outputCache.Clear(p.namespace)

	if errRunner != nil {
		return errRunner
	}
	if errWriter != nil {
		return errWriter
	}
	if errOutputCache != nil {
		return errOutputCache
	}

	return nil
}

func (p *project) triggerCleanup() error {
	if err := p.doCleanup(); err != nil {
		log.Printf("cleanup failed for %q: %s", p.namespace, err)
		// Schedule this instance for recycling.
		p.dead.Store(true)
		return err
	}
	return nil
}

func (p *project) run(ctx context.Context, options *types.CommandOptions) (types.ExitCode, error) {
	timeout := time.Duration(options.ComputeTimeout)
	ctx, done := context.WithTimeout(ctx, timeout)
	defer done()

	var lastErr error
	for i := int64(0); i < 3; i++ {
		if lastErr == context.Canceled ||
			lastErr == context.DeadlineExceeded {
			break
		}
		if err := ctx.Err(); err != nil {
			lastErr = err
			break
		}

		deadline, _ := ctx.Deadline()
		remaining := time.Until(deadline)
		// Bail out if we have got less than half the timeout left.
		if timeout/remaining > 2 {
			break
		}

		isFirstAttempt := i == 0
		if !isFirstAttempt || p.needsRunnerSetup(deadline) {
			if err := p.setupRunner(ctx, options.ImageName); err != nil {
				lastErr = err
				continue
			}
		}
		code, err := p.tryRun(ctx, options)
		if err == nil {
			return code, nil
		}
		lastErr = err
		continue
	}
	return -1, lastErr
}

func (p *project) StartInBackground(imageName sharedTypes.ImageName) {
	if p.needsRunnerSetup(time.Now().Add(time.Minute)) {
		go func() {
			ctx, done := context.WithTimeout(context.Background(), time.Second*10)
			defer done()
			// We can ignore setup errors. They are already logged.
			_ = p.setupRunner(ctx, imageName)
		}()
	}
}

func (p *project) needsRunnerSetup(deadline time.Time) bool {
	p.runnerSetupMux.RLock()
	defer p.runnerSetupMux.RUnlock()
	return p.needsRunnerSetupLocked(deadline)
}

func (p *project) needsRunnerSetupLocked(deadline time.Time) bool {
	return p.runnerSetupValidUntil.Before(deadline)
}

func (p *project) setupRunner(ctx context.Context, imageName sharedTypes.ImageName) error {
	pending, err := p.triggerRunnerSetup(ctx, imageName)
	if err != nil {
		return err
	}
	err = pending.Wait(ctx)
	if err != nil {
		log.Printf("setup failed for %q: %s", p.namespace, err)
		// Schedule this instance for recycling.
		p.dead.Store(true)
		return err
	}
	return nil
}

func (p *project) triggerRunnerSetup(ctx context.Context, imageName sharedTypes.ImageName) (pendingOperation.PendingOperation, error) {
	p.runnerSetupMux.Lock()
	defer p.runnerSetupMux.Unlock()

	if err := p.checkIsDead(); err != nil {
		// There is no need to perform setup for a dead project.
		return nil, err
	}

	pending := p.pendingRunnerSetup
	if pending != nil {
		// Someone else won the race for triggering a new setup operation.
		return pending, nil
	}

	pending = pendingOperation.TrackOperationWithCancel(
		ctx,
		func(setupCtx context.Context) error {
			validUntil, err := p.runner.Setup(setupCtx, p.namespace, imageName)
			p.runnerSetupMux.Lock()
			defer p.runnerSetupMux.Unlock()
			if p.pendingRunnerSetup == pending {
				p.pendingRunnerSetup = nil
				if err == nil {
					p.runnerSetupValidUntil = *validUntil
				}
			}
			return err
		},
	)
	p.pendingRunnerSetup = pending
	return pending, nil
}

func (p *project) tryRun(ctx context.Context, options *types.CommandOptions) (types.ExitCode, error) {
	// Block new setup calls while we use the container.
	p.runnerSetupMux.RLock()
	defer p.runnerSetupMux.RUnlock()

	if err := p.checkIsDead(); err != nil {
		return -1, err
	}
	if pendingSetup := p.pendingRunnerSetup; pendingSetup != nil {
		if err := pendingSetup.Wait(ctx); err != nil {
			return -1, err
		}
	}

	deadline, _ := ctx.Deadline()
	if p.needsRunnerSetupLocked(deadline) {
		return -1, errors.New("runner setup expired")
	}

	// Update the timeout
	options.ComputeTimeout = sharedTypes.ComputeTimeout(time.Until(deadline))

	return p.runner.Run(ctx, p.namespace, options)
}
