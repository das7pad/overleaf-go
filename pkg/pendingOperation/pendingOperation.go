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

package pendingOperation

import (
	"context"
	"sync"

	"github.com/das7pad/overleaf-go/pkg/errors"
)

var errOperationStillPending = errors.New("operation is still pending")

type PendingOperation interface {
	Done() <-chan struct{}
	Err() error
	IsPending() bool
	Failed() bool
	Wait(ctx context.Context) error
}

type pendingOperation struct {
	c   chan struct{}
	err error
	mu  sync.Mutex
}

func (c *pendingOperation) Done() <-chan struct{} {
	return c.c
}

func (c *pendingOperation) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

func (c *pendingOperation) IsPending() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err == errOperationStillPending
}

func (c *pendingOperation) Failed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err != nil && c.err != errOperationStillPending
}

func (c *pendingOperation) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		if !c.IsPending() {
			return c.Err()
		}
		return ctx.Err()
	case <-c.Done():
		return c.Err()
	}
}

func (c *pendingOperation) setErr(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.err = err
	close(c.c)
}

func newPendingOperation() (PendingOperation, func(err error)) {
	c := make(chan struct{})
	pending := &pendingOperation{
		c:   c,
		err: errOperationStillPending,
	}
	return pending, pending.setErr
}

func TrackOperation(fn func() error) PendingOperation {
	p, setErr := newPendingOperation()
	go func() {
		setErr(fn())
	}()
	return p
}
