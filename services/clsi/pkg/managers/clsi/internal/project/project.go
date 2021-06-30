// Golang port of the Overleaf clsi service
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
	"log"
	"os"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/das7pad/clsi/pkg/constants"
	"github.com/das7pad/clsi/pkg/errors"
	"github.com/das7pad/clsi/pkg/types"
)

type Project interface {
	IsDead() bool
	IsHealthy(activeThreshold time.Time) bool

	Cleanup() PendingOperation

	CleanupUnlessHealthy(
		ctx context.Context,
		activeThreshold time.Time,
	) error

	ClearCache(ctx context.Context) error

	Compile(
		ctx context.Context,
		request *types.CompileRequest,
		response *types.CompileResponse,
	) error

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
	projectId primitive.ObjectID,
	userId primitive.ObjectID,
	m *managers,
	options *types.Options,
) (Project, error) {
	namespace := types.Namespace(projectId.Hex() + "-" + userId.Hex())

	createPaths := []string{
		string(options.CacheBaseDir.NamespacedCacheDir(namespace)),
		string(options.CompileBaseDir.CompileDir(namespace)),
		string(options.OutputBaseDir.OutputDir(namespace)),
		options.OutputBaseDir.OutputDir(namespace).CompileOutput(),
	}
	for _, path := range createPaths {
		// Any parent directories have been created during manager init.
		if err := os.Mkdir(path, 0755); err != nil && !os.IsExist(err) {
			return nil, err
		}
	}

	initialState := types.SyncStateCleared
	if state, err := m.writer.GetState(namespace); err == nil {
		initialState = state
	}

	return &project{
		dead:       false,
		lastAccess: time.Now(),
		namespace:  namespace,
		projectId:  projectId,

		state:                 initialState,
		stateMux:              sync.RWMutex{},
		runnerSetupValidUntil: time.Now(),
		runnerSetupMux:        sync.RWMutex{},
		compileMux:            sync.Mutex{},

		managers: m,
	}, nil
}

type project struct {
	dead       bool
	lastAccess time.Time
	namespace  types.Namespace
	projectId  primitive.ObjectID

	state          types.SyncState
	stateMux       sync.RWMutex
	pendingCleanup PendingOperation

	runnerSetupValidUntil time.Time
	runnerSetupMux        sync.RWMutex
	pendingRunnerSetup    PendingOperationWithCancel

	compileMux     sync.Mutex
	pendingCompile PendingOperationWithCancel

	*managers
}

func (p *project) IsDead() bool {
	return p.dead
}

func (p *project) IsHealthy(activeThreshold time.Time) bool {
	return !p.IsDead() && p.lastAccess.After(activeThreshold)
}

func (p *project) Cleanup() PendingOperation {
	p.dead = true
	pending := p.triggerCleanup()
	return pending
}

func (p *project) CleanupUnlessHealthy(ctx context.Context, activeThreshold time.Time) error {
	if p.IsHealthy(activeThreshold) {
		return nil
	}
	return p.Cleanup().Wait(ctx)
}

func (p *project) ClearCache(ctx context.Context) error {
	pending := p.triggerCleanup()
	err := pending.Wait(ctx)
	if err != nil {
		log.Printf("cleanup failed for %q: %s", p.namespace, err)
		// Schedule this instance for recycling.
		p.dead = true
		return err
	}

	// Take write lock
	p.stateMux.Lock()
	defer p.stateMux.Unlock()

	if nextPending := p.pendingCleanup; nextPending != pending {
		// Someone else won the race.
		return nil
	}

	p.pendingCleanup = nil
	return nil
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
	// Cheap check. The lock will block one of the racing compile requests.
	if p.pendingCompile != nil {
		return &errors.AlreadyCompilingError{}
	}

	p.compileMux.Lock()
	defer p.compileMux.Unlock()

	// Double check after acquiring the lock.
	if p.pendingCompile != nil {
		return &errors.AlreadyCompilingError{}
	}

	p.pendingCompile = trackOperationWithCancel(
		ctx,
		func(compileCtx context.Context) error {
			return p.doCompile(compileCtx, request, response)
		},
	)
	err := p.pendingCompile.Wait(ctx)
	p.pendingCompile = nil
	return err
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

	cmdOutputFiles, err := p.latexRunner.Run(
		ctx,
		p.run,
		p.namespace,
		request,
		response,
	)
	if err != nil {
		return err
	}

	response.Timings.Output.Begin()
	outputFiles, hasOutputPDF, err := p.outputCache.SaveOutputFiles(
		ctx,
		cmdOutputFiles,
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

func (p *project) StopCompile(_ context.Context) error {
	pending := p.pendingCompile
	if pending == nil {
		return nil
	}
	pending.Cancel()
	<-pending.Done()

	p.compileMux.Lock()
	defer p.compileMux.Unlock()

	if nextPending := p.pendingCompile; nextPending == pending {
		// Someone else won the race.
		return nil
	}
	p.pendingCompile = nil
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
	p.lastAccess = time.Now()
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

func (p *project) checkIsDead() error {
	if p.IsDead() {
		return &errors.InvalidStateError{
			Msg:         "project is dead",
			Recoverable: true,
		}
	}
	return nil
}

func (p *project) checkStateExpectAnyContent() error {
	if err := p.checkIsDead(); err != nil {
		return err
	}
	if p.state == types.SyncStateCleared {
		return &errors.InvalidStateError{
			Msg:         "project contents are missing",
			Recoverable: false,
		}
	}
	return nil
}

func (p *project) checkSyncState(syncType types.SyncType, state types.SyncState) error {
	if syncType == types.SyncTypeFull ||
		syncType == types.SyncTypeFullIncremental {
		// SyncTypeFull and SyncTypeFullIncremental overwrite everything.
		return nil
	}

	needsFullSync := p.state == "" || p.state == types.SyncStateCleared
	if needsFullSync {
		return &errors.InvalidStateError{
			Msg:         "local sync type empty and incoming syncType!=full",
			Recoverable: false,
		}
	}

	if p.state != state {
		return &errors.InvalidStateError{
			Msg:         "local sync state differs remote state",
			Recoverable: false,
		}
	}
	return nil
}

func (p *project) doCleanup() error {
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

func (p *project) triggerCleanup() PendingOperation {
	pendingBeforeLock := p.pendingCleanup
	if pendingBeforeLock != nil && pendingBeforeLock.IsPending() {
		return pendingBeforeLock
	}

	if compile := p.pendingCompile; compile != nil {
		compile.Cancel()
	}
	if setup := p.pendingRunnerSetup; setup != nil {
		setup.Cancel()
	}

	// Take write lock
	p.stateMux.Lock()
	defer p.stateMux.Unlock()
	p.state = types.SyncStateCleared

	if nextPending := p.pendingCleanup; nextPending != pendingBeforeLock {
		// Someone else won the race.
		return nextPending
	}

	p.pendingCleanup = trackOperation(p.doCleanup)
	return p.pendingCleanup
}

func (p *project) run(ctx context.Context, options *types.CommandOptions) (types.ExitCode, error) {
	timeout := time.Duration(options.Timeout)
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
		remaining := deadline.Sub(time.Now())
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
		if code, err := p.tryRun(ctx, options); err == nil {
			return code, nil
		} else {
			lastErr = err
			continue
		}
	}
	return -1, lastErr
}

func (p *project) needsRunnerSetup(deadline time.Time) bool {
	return p.runnerSetupValidUntil.Before(deadline)
}

func (p *project) setupRunner(ctx context.Context, imageName types.ImageName) error {
	pending, err := p.triggerRunnerSetup(ctx, imageName)
	if err != nil {
		return err
	}
	err = pending.Wait(ctx)
	if err != nil {
		log.Printf("setup failed for %q: %s", p.namespace, err)
		// Schedule this instance for recycling.
		p.dead = true
		return err
	}

	// Take write lock
	p.runnerSetupMux.Lock()
	defer p.runnerSetupMux.Unlock()

	if nextPending := p.pendingRunnerSetup; nextPending != pending {
		// Someone else won the race.
		return nil
	}

	p.pendingRunnerSetup = nil
	return nil
}

func (p *project) triggerRunnerSetup(ctx context.Context, imageName types.ImageName) (PendingOperation, error) {
	pendingBeforeLock := p.pendingRunnerSetup
	if pendingBeforeLock != nil && pendingBeforeLock.IsPending() {
		return pendingBeforeLock, nil
	}

	// Take write lock
	p.runnerSetupMux.Lock()
	defer p.runnerSetupMux.Unlock()

	if err := p.checkIsDead(); err != nil {
		// There is no need to perform setup for a dead project.
		return nil, err
	}

	nextPending := p.pendingRunnerSetup
	if nextPending != nil && nextPending != pendingBeforeLock {
		// Someone else won the race.
		return nextPending, nil
	}

	p.pendingRunnerSetup = trackOperationWithCancel(
		ctx,
		func(setupCtx context.Context) error {
			validUntil, err := p.runner.Setup(setupCtx, p.namespace, imageName)
			if err == nil {
				p.runnerSetupValidUntil = *validUntil
			}
			return err
		},
	)
	return p.pendingRunnerSetup, nil
}

func (p *project) tryRun(ctx context.Context, options *types.CommandOptions) (types.ExitCode, error) {
	p.runnerSetupMux.RLock()
	defer p.runnerSetupMux.RUnlock()

	if err := p.checkIsDead(); err != nil {
		return -1, err
	}
	pendingSetup := p.pendingRunnerSetup
	if pendingSetup != nil {
		if err := pendingSetup.Wait(ctx); err != nil {
			return -1, err
		}
	}

	deadline, _ := ctx.Deadline()
	if p.needsRunnerSetup(deadline) {
		return -1, errors.New("runner setup expired")
	}

	// Update the timeout
	options.Timeout = types.Timeout(deadline.Sub(time.Now()))

	return p.runner.Run(ctx, p.namespace, options)
}
