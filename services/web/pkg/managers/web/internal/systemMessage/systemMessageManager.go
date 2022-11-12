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

package systemMessage

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"

	"github.com/das7pad/overleaf-go/pkg/models/systemMessage"
	"github.com/das7pad/overleaf-go/pkg/pendingOperation"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
)

type Manager interface {
	GetAllCached(ctx context.Context, userId sharedTypes.UUID) ([]systemMessage.Full, error)
	GetAllCachedOnly(userId sharedTypes.UUID) ([]systemMessage.Full, bool)
}

type manager struct {
	sm systemMessage.Manager

	l       sync.RWMutex
	pending pendingOperation.PendingOperation
	expires time.Time
	cached  []systemMessage.Full
}

var noMessages = make([]systemMessage.Full, 0)

func New(db *pgxpool.Pool) Manager {
	return &manager{
		sm:     systemMessage.New(db),
		cached: noMessages,
	}
}

func (m *manager) GetAllCached(ctx context.Context, userId sharedTypes.UUID) ([]systemMessage.Full, error) {
	if userId.IsZero() {
		// Hide messages for logged out users.
		return noMessages, nil
	}
	if messages, ok := m.fast(); ok {
		return messages, nil
	}
	return m.slow(ctx)
}

func (m *manager) GetAllCachedOnly(userId sharedTypes.UUID) ([]systemMessage.Full, bool) {
	if userId.IsZero() {
		// Hide messages for logged out users.
		return noMessages, true
	}
	if messages, ok := m.fast(); ok {
		return messages, true
	}
	return nil, false
}

func (m *manager) fast() ([]systemMessage.Full, bool) {
	m.l.RLock()
	defer m.l.RUnlock()
	if m.expires.After(time.Now()) {
		return m.cached, true
	}
	return nil, false
}

func (m *manager) slow(ctx context.Context) ([]systemMessage.Full, error) {
	m.l.Lock()
	if m.expires.After(time.Now()) {
		defer m.l.Unlock()
		// Another goroutine refreshed the cache already.
		return m.cached, nil
	}
	pending := m.pending
	if pending == nil {
		pending = pendingOperation.TrackOperation(m.refresh)
		m.pending = pending
	}
	m.l.Unlock()

	err := pending.Wait(ctx)

	m.l.Lock()
	defer m.l.Unlock()
	if m.pending == pending {
		m.pending = nil
	}
	if err != nil {
		return nil, err
	}
	return m.cached, nil
}

func (m *manager) refresh() error {
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	messages, err := m.sm.GetAll(ctx)
	if err != nil {
		return err
	}
	jitter := time.Duration(rand.Int63n(int64(2 * time.Second)))
	m.l.Lock()
	defer m.l.Unlock()
	m.cached = messages
	m.expires = time.Now().Add(10*time.Second + jitter)
	return nil
}
