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

package clientTracking

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/das7pad/overleaf-go/pkg/errors"
	"github.com/das7pad/overleaf-go/pkg/pubSub/channel"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/managers/realTime/internal/editorEvents"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

type Manager interface {
	Disconnect(client *types.Client) bool
	GetConnectedClients(ctx context.Context, client *types.Client) (json.RawMessage, error)
	Connect(ctx context.Context, client *types.Client, fetchConnectedUsers bool) json.RawMessage
	RefreshClientPositions(ctx context.Context, client map[sharedTypes.UUID]editorEvents.Clients) error
	UpdatePosition(ctx context.Context, client *types.Client, position types.ClientPosition) error
}

func New(client redis.UniversalClient, c channel.Writer) Manager {
	m := &manager{
		redisClient: client,
		c:           c,
	}
	for i := 0; i < 256; i++ {
		m.pcc[i].pending = make(map[sharedTypes.UUID]*pendingConnectedClients)
	}
	return m
}

type manager struct {
	redisClient redis.UniversalClient
	c           channel.Writer
	pcc         [256]pendingConnectedClientsManager
}

const (
	ProjectExpiry    = 4 * 24 * time.Hour
	UserExpiry       = 15 * time.Minute
	RefreshUserEvery = UserExpiry - 1*time.Minute
)

func (m *manager) Disconnect(client *types.Client) bool {
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()

	n, err := m.deleteClientPosition(ctx, client)
	if err != nil || n > 0 {
		if errNotify := m.notifyDisconnected(ctx, client); errNotify != nil {
			err = errors.Merge(err, errNotify)
		}
	}
	if err != nil {
		err = errors.Tag(err, "disconnect connected client")
		log.Printf("%s/%s: %s", client.ProjectId, client.PublicId, err)
		return true
	}
	return n == 0
}

func (m *manager) Connect(ctx context.Context, client *types.Client, fetchConnectedUsers bool) json.RawMessage {
	notify, clients, err := m.updateClientPosition(
		ctx, client, types.ClientPosition{}, fetchConnectedUsers,
	)
	if err != nil || notify {
		if errNotify := m.notifyConnected(ctx, client); errNotify != nil {
			err = errors.Merge(errNotify, err)
		}
	}
	if err != nil {
		err = errors.Tag(err, "initialize connected client")
		log.Printf("%s/%s: %s", client.ProjectId, client.PublicId, err)
	}
	return clients
}

func (m *manager) UpdatePosition(ctx context.Context, client *types.Client, p types.ClientPosition) error {
	if err := m.notifyUpdated(ctx, client, p); err != nil {
		return err
	}
	if _, _, err := m.updateClientPosition(ctx, client, p, false); err != nil {
		return err
	}
	return nil
}

type pendingConnectedClientsManager struct {
	mu      sync.RWMutex
	pending map[sharedTypes.UUID]*pendingConnectedClients
}

func (m *pendingConnectedClientsManager) get(projectId sharedTypes.UUID) (*pendingConnectedClients, bool) {
	m.mu.RLock()
	pending, ok := m.pending[projectId]
	if ok {
		pending.shared.CompareAndSwap(false, true)
	}
	m.mu.RUnlock()
	if !ok {
		m.mu.Lock()
		pending, ok = m.pending[projectId]
		if ok {
			pending.shared.CompareAndSwap(false, true)
		} else {
			pending = &pendingConnectedClients{done: make(chan struct{})}
			m.pending[projectId] = pending
		}
		m.mu.Unlock()
		return pending, !ok
	}
	return pending, false
}

func (m *pendingConnectedClientsManager) delete(projectId sharedTypes.UUID) {
	m.mu.Lock()
	delete(m.pending, projectId)
	m.mu.Unlock()
}

type pendingConnectedClients struct {
	shared atomic.Bool
	done   chan struct{}
	// clients contains serialized types.ConnectedClients
	clients json.RawMessage
	err     error
}
