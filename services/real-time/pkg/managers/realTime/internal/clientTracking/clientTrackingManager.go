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
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/das7pad/overleaf-go/pkg/pubSub/channel"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/managers/realTime/internal/editorEvents"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

type Manager interface {
	GetConnectedClients(ctx context.Context, client *types.Client) (json.RawMessage, error)
	RefreshClientPositions(ctx context.Context, client map[sharedTypes.UUID]editorEvents.Clients) error
	UpdatePosition(ctx context.Context, client *types.Client, position types.ClientPosition) error
	FlushRoomChanges(projectId sharedTypes.UUID, rc editorEvents.RoomChanges)
}

func New(client redis.UniversalClient, c channel.Writer) Manager {
	m := manager{
		redisClient: client,
		c:           c,
	}
	for i := 0; i < 256; i++ {
		m.pcc[i].pending = make(map[sharedTypes.UUID]*pendingConnectedClients)
	}
	return &m
}

type manager struct {
	redisClient redis.UniversalClient
	c           channel.Writer
	pcc         [256]pendingConnectedClientsManager
}

func (m *manager) FlushRoomChanges(projectId sharedTypes.UUID, rcs editorEvents.RoomChanges) {
	added := 0
	removed := 0
	for _, rcc := range rcs {
		if rcc.IsJoin {
			added++
		} else {
			removed++
		}
	}
	hSet := make([]interface{}, added*4)
	hDel := make([]string, removed*2)
	added = 0
	removed = 0
	for _, rc := range rcs {
		if rc.IsJoin {
			userBlob, err := json.Marshal(types.ConnectingConnectedClient{
				DisplayName: rc.DisplayName,
			})
			if err != nil {
				log.Printf(
					"%s/%s: failed to serialize connectedClient: %s",
					projectId, rc.PublicId, err,
				)
				continue
			}
			hSet[added] = string(rc.PublicId)
			hSet[added+1] = userBlob
			hSet[added+2] = string(rc.PublicId) + ":age"
			hSet[added+3] = string(rc.PublicId)[:types.PublicIdTsPrefixLength]
			added += 4
		} else {
			hDel[removed] = string(rc.PublicId)
			hDel[removed+1] = string(rc.PublicId) + ":age"
			removed += 2
		}
	}

	var msg channel.Message
	{
		body, err := json.Marshal(rcs)
		if err != nil {
			log.Printf(
				"%s: failed to serialize room changes: %s", projectId, err,
			)
		} else {
			var source sharedTypes.PublicId
			if len(rcs) == 1 {
				source = rcs[0].PublicId
			}
			msg = &sharedTypes.EditorEvent{
				Source:  source,
				Message: sharedTypes.ClientTrackingBatch,
				RoomId:  projectId,
				Payload: body,
			}
		}
	}

	projectKey := getProjectKey(projectId)
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	_, err := m.redisClient.TxPipelined(ctx, func(p redis.Pipeliner) error {
		if added > 0 {
			p.HSet(ctx, projectKey, hSet[:added]...)
			p.Expire(ctx, projectKey, ProjectExpiry)
		}
		if removed > 0 {
			p.HDel(ctx, projectKey, hDel...)
		}
		if msg != nil {
			if _, err := m.c.PublishVia(ctx, p, msg); err != nil {
				log.Printf("%s: publish room changes: %s", projectId, err)
			}
		}
		return nil
	})
	if err != nil {
		log.Printf(
			"%s: failed to flush connectedClient changes: %s", projectId, err,
		)
	}
}

const (
	ProjectExpiry    = time.Hour
	UserExpiry       = 15 * time.Minute
	RefreshUserEvery = UserExpiry - 1*time.Minute
)

func (m *manager) UpdatePosition(ctx context.Context, client *types.Client, p types.ClientPosition) error {
	if err := m.notifyUpdated(ctx, client, p); err != nil {
		return err
	}
	if err := m.updateClientPosition(ctx, client, p); err != nil {
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
	m.mu.RUnlock()
	if !ok {
		m.mu.Lock()
		pending, ok = m.pending[projectId]
		if !ok {
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
	done chan struct{}
	// clients contains serialized types.ConnectedClients
	clients json.RawMessage
	err     error
}
