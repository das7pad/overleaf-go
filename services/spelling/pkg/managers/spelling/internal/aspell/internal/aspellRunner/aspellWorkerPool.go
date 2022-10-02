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

package aspellRunner

import (
	"context"
	"sync"
	"time"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/services/spelling/pkg/types"
)

type WorkerPool interface {
	CheckWords(ctx context.Context, language types.SpellCheckLanguage, words []string) ([]string, error)
	Close()
}

const (
	MaxRequests        = 100 * 1024
	MaxWorkers         = 32
	MaxIdleTime        = 30 * time.Second
	MaxRequestDuration = 10 * time.Second
)

func newWorkerPool() WorkerPool {
	t := time.NewTicker(MaxIdleTime / 2)
	sem := make(chan struct{}, MaxWorkers)
	for i := 0; i < MaxWorkers; i++ {
		sem <- struct{}{}
	}
	wp := workerPool{
		slots:         make([]Worker, MaxWorkers),
		freeSlots:     MaxWorkers,
		sem:           sem,
		cleanupTicker: t,
	}
	go func() {
		for now := range t.C {
			wp.killIdleWorkersOnce(now.Add(-MaxIdleTime))
		}
	}()
	return &wp
}

type workerPool struct {
	l             sync.Mutex
	sem           chan struct{}
	slots         []Worker
	freeSlots     int8
	cleanupTicker *time.Ticker
}

func (wp *workerPool) CheckWords(ctx context.Context, language types.SpellCheckLanguage, words []string) ([]string, error) {
	<-wp.sem
	defer func() { wp.sem <- struct{}{} }()
	w, err := wp.getWorker(language)
	if err != nil {
		return nil, err
	}
	lines, err := w.CheckWords(ctx, words)
	wp.returnWorker(w, err)
	return lines, err
}

func (wp *workerPool) Close() {
	wp.cleanupTicker.Stop()
	for {
		wp.killIdleWorkersOnce(time.Now())
		wp.l.Lock()
		if wp.freeSlots == MaxWorkers {
			break
		}
		wp.l.Unlock()
	}
}

func (wp *workerPool) killIdleWorkersOnce(threshold time.Time) {
	idle := make([]Worker, 0, MaxWorkers)
	wp.l.Lock()
	for i, w := range wp.slots {
		if w == nil {
			continue
		}
		if w.LastUsed().Before(threshold) {
			idle = append(idle, w)
			wp.slots[i] = nil
			wp.freeSlots++
		}
	}
	wp.l.Unlock()
	for _, w := range idle {
		w.Kill()
	}
}

func (wp *workerPool) getWorker(language types.SpellCheckLanguage) (Worker, error) {
	wp.l.Lock()
	for i, w := range wp.slots {
		if w == nil {
			continue
		}
		if w.Language() == language {
			wp.slots[i] = nil
			wp.l.Unlock()
			return w, nil
		}
	}
	if wp.freeSlots == 0 {
		// Steal a slot from another language.
		// NOTE: wp.sem guarantees that at least one slot is populated.
		var takeOverAt int
		var oldest time.Time
		for i, w := range wp.slots {
			if w == nil {
				continue
			}
			if oldest.IsZero() || w.LastUsed().Before(oldest) {
				oldest = w.LastUsed()
				takeOverAt = i
			}
		}
		w := wp.slots[takeOverAt]
		wp.slots[takeOverAt] = nil
		wp.l.Unlock()
		w.Kill()
	} else {
		wp.freeSlots--
		wp.l.Unlock()
	}
	w, err := newAspellWorker(language)
	if err != nil {
		wp.l.Lock()
		wp.freeSlots++
		wp.l.Unlock()
		return nil, errors.Tag(err, "cannot create worker")
	}
	return w, nil
}

func (wp *workerPool) returnWorker(w Worker, err error) {
	alive := err == nil
	keep := alive && w.Count() < MaxRequests
	wp.l.Lock()
	if keep {
		for i, slot := range wp.slots {
			if slot == nil {
				wp.slots[i] = w
				break
			}
		}
	} else {
		wp.freeSlots++
	}
	wp.l.Unlock()
	if !keep && alive {
		w.Kill()
	}
}
