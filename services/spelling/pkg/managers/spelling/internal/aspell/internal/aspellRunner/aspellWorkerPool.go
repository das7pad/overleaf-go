// Golang port of the Overleaf spelling service
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

package aspellRunner

import (
	"context"
	"sync"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

type WorkerPool interface {
	CheckWords(
		ctx context.Context,
		language string,
		words []string,
	) ([]string, error)
}

const (
	MaxRequests    = 100 * 1024
	MaxWorkers     = 32
	MaxIdleTime    = 1 * time.Second
	MaxRequestTime = 1 * time.Minute
)

var (
	errIdleWorker    = errors.New("idle worker")
	errTimedOut      = errors.New("spell check timed out")
	errPoolFull      = errors.New("maximum number of workers already running")
	errCannotAcquire = errors.New("cannot acquire worker")
	errMaxAgeHit     = errors.New("too many requests")
)

func newWorkerPool() WorkerPool {
	return &workerPool{
		createWorker: newAspellWorker,
		l:            &sync.Mutex{},
	}
}

type workerPoolEntry struct {
	Worker
	l *sync.Mutex
	t *time.Timer
}

func (e *workerPoolEntry) Acquire() bool {
	e.l.Lock()
	defer e.l.Unlock()
	ok := e.TransitionState(Ready, Busy)
	if ok && e.t != nil {
		e.t.Stop()
		e.t = nil
	}
	return ok
}

func (e *workerPoolEntry) Release() bool {
	e.l.Lock()
	defer e.l.Unlock()
	e.t = time.AfterFunc(MaxIdleTime*10, func() {
		e.l.Lock()
		defer e.l.Unlock()
		if !e.TransitionState(Ready, Closing) {
			// Worker is back in use as we acquired the lock.
			return
		}
		e.Shutdown(errIdleWorker)
		e.t = nil
	})
	return e.TransitionState(Busy, Ready)
}

type workerPool struct {
	createWorker func(language string) (Worker, error)
	processPool  []*workerPoolEntry
	l            *sync.Mutex
}

func (wp *workerPool) CheckWords(ctx context.Context, language string, words []string) ([]string, error) {
	w, err := wp.getWorker(language)
	if err != nil {
		return nil, err
	}
	defer wp.returnWorker(w)

	ctx, cancel := context.WithTimeout(ctx, MaxRequestTime)
	defer cancel()

	go func() {
		<-ctx.Done()
		if ctx.Err() == context.Canceled {
			return
		}
		w.Kill(errTimedOut)
	}()

	return w.CheckWords(ctx, words)
}

func (wp *workerPool) getWorker(language string) (*workerPoolEntry, error) {
	wp.l.Lock()
	defer wp.l.Unlock()

	for _, w := range wp.processPool {
		if w.Language() == language && w.Acquire() {
			return w, nil
		}
	}
	if len(wp.processPool) == MaxWorkers {
		return nil, errPoolFull
	}
	var w *workerPoolEntry
	actualWorker, err := wp.createWorker(language)
	if err != nil {
		return nil, errors.Tag(err, "cannot create worker")
	}
	w = &workerPoolEntry{Worker: actualWorker, l: &sync.Mutex{}}

	if !w.Acquire() {
		err = w.Error()
		if err == nil {
			return nil, errCannotAcquire
		}
		return nil, errors.Tag(w.Error(), "worker died before use")
	}
	wp.processPool = append(wp.processPool, w)
	go func() {
		<-w.Done()
		if w.t != nil {
			w.t.Stop()
			w.t = nil
		}
		wp.removeFromPool(w)
	}()
	return w, nil
}

func (wp *workerPool) removeFromPool(w *workerPoolEntry) {
	wp.l.Lock()
	defer wp.l.Unlock()

	var idx int
	for i, poolEntry := range wp.processPool {
		if w == poolEntry {
			idx = i
			break
		}
	}
	poolSize := len(wp.processPool)
	lastPoolEntry := wp.processPool[poolSize-1]
	wp.processPool[idx] = lastPoolEntry
	wp.processPool = wp.processPool[:poolSize-1]
}

func (wp *workerPool) returnWorker(w *workerPoolEntry) {
	if w.Count() > MaxRequests {
		w.Shutdown(errMaxAgeHit)
		return
	}
	w.Release()
}
