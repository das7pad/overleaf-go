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

package pendingOperation

import (
	"context"

	"github.com/das7pad/overleaf-go/services/real-time/pkg/errors"
)

var OperationStillPending = errors.New("operation is still pending")

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
}

func (c *pendingOperation) Done() <-chan struct{} {
	return c.c
}

func (c *pendingOperation) Err() error {
	return c.err
}

func (c *pendingOperation) IsPending() bool {
	return c.err == OperationStillPending
}

func (c *pendingOperation) Failed() bool {
	return !c.IsPending() && c.err != nil
}

func (c *pendingOperation) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		if !c.IsPending() {
			return c.err
		}
		return ctx.Err()
	case <-c.Done():
		return c.Err()
	}
}

func (c *pendingOperation) setErr(err error) {
	c.err = err
	close(c.c)
}

func newPendingOperation() (PendingOperation, func(err error)) {
	c := make(chan struct{})
	pending := &pendingOperation{
		c:   c,
		err: OperationStillPending,
	}
	return pending, pending.setErr
}
