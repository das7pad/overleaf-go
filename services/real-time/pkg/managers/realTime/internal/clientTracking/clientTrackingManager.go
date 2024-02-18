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
	"sort"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/das7pad/overleaf-go/pkg/pubSub/channel"
	"github.com/das7pad/overleaf-go/pkg/sharedTypes"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/managers/realTime/internal/editorEvents"
	"github.com/das7pad/overleaf-go/services/real-time/pkg/types"
)

type Manager interface {
	GetConnectedClients(ctx context.Context, client *types.Client) (json.RawMessage, error)
	RefreshClientPositions(ctx context.Context, rooms editorEvents.Rooms) error
	UpdatePosition(ctx context.Context, client *types.Client, position types.ClientPosition) error
	FlushRoomChanges(projectId sharedTypes.UUID, rc types.RoomChanges)
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

type flushRoomChangesCached struct {
	drop        []int
	hSet        [][]byte
	hDel        [][]byte
	nameBuf     []byte
	publicIdBuf []byte
}

func (f *flushRoomChangesCached) getDrop(n int) []int {
	if cap(f.drop) < n {
		f.drop = make([]int, n)
	}
	return f.drop[:0]
}

func (f *flushRoomChangesCached) getHSet(n int) [][]byte {
	if cap(f.hSet) < n {
		f.hSet = make([][]byte, n)
	}
	return f.hSet[:n]
}

func (f *flushRoomChangesCached) getHDel(n int) [][]byte {
	if cap(f.hDel) < n {
		f.hDel = make([][]byte, n)
	}
	return f.hDel[:n]
}

func (f *flushRoomChangesCached) getNameBuf(n int) []byte {
	if cap(f.nameBuf) < n {
		f.nameBuf = make([]byte, n)
	}
	return f.nameBuf[:0]
}

func (f *flushRoomChangesCached) getPublicIdBuf(n int) []byte {
	if cap(f.publicIdBuf) < n {
		f.publicIdBuf = make([]byte, n)
	}
	return f.publicIdBuf[:0]
}

var emptyConnectingConnectedClient = []byte("{}")

var flushRoomChangesPool = sync.Pool{New: func() any {
	return &flushRoomChangesCached{}
}}

func (m *manager) FlushRoomChanges(projectId sharedTypes.UUID, rcs types.RoomChanges) {
	f := flushRoomChangesPool.Get().(*flushRoomChangesCached)
	defer flushRoomChangesPool.Put(f)

	added := 0
	removed := 0
	namesSize := 0
	for _, rc := range rcs {
		if rc.IsJoin != 0 {
			added++
			namesSize += len(rc.DisplayName)
		} else {
			removed++
		}
	}
	drop := f.getDrop(removed)
	if removed > 0 {
		for i, rc := range rcs {
			if rc.IsJoin != 0 {
				continue
			}
			for j, other := range rcs[:i] {
				if other.PublicId == rc.PublicId {
					drop = append(drop, j)
					namesSize -= len(other.DisplayName)
					break
				}
			}
		}
		if len(drop) > 0 {
			sort.Ints(drop)
			added -= len(drop)
			// sort.SearchInts(a,i) returns idx<=len(a), make it idx<len(drop)
			drop = append(drop, len(rcs))
		}
	}
	hSet := f.getHSet(added * 4)
	hDel := f.getHDel(removed * 2)
	nameBuf := f.getNameBuf(namesSize + added*len(`{"n":""}`))
	const publicIdBufItemLen = types.PublicIdLength + len(":age")
	publicIdBuf := f.getPublicIdBuf((added + removed) * publicIdBufItemLen)
	for i, rc := range rcs {
		if rc.IsJoin != 0 && len(drop) > 0 && drop[sort.SearchInts(drop, i)] == i {
			continue
		}
		publicIdBuf = append(publicIdBuf, rc.PublicId...)
		publicIdBuf = append(publicIdBuf, ageSuffix...)
	}

	addedIdx := 0
	removedIdx := 0
	for i, rc := range rcs {
		off := (addedIdx + removedIdx) * publicIdBufItemLen
		if rc.IsJoin != 0 {
			if len(drop) > 0 && drop[sort.SearchInts(drop, i)] == i {
				continue
			}
			hSet[addedIdx*4] = publicIdBuf[off : off+types.PublicIdLength]
			if len(rc.DisplayName) == 0 {
				hSet[addedIdx*4+1] = emptyConnectingConnectedClient
			} else {
				nameBuf = types.ConnectingConnectedClient{
					DisplayName: rc.DisplayName,
				}.Append(nameBuf[len(nameBuf):])
				hSet[addedIdx*4+1] = nameBuf
			}
			hSet[addedIdx*4+2] = publicIdBuf[off : off+publicIdBufItemLen]
			hSet[addedIdx*4+3] = publicIdBuf[off : off+types.PublicIdTsPrefixLength]
			addedIdx++
		} else {
			hDel[removedIdx*2] = publicIdBuf[off : off+types.PublicIdLength]
			hDel[removedIdx*2+1] = publicIdBuf[off : off+publicIdBufItemLen]
			removedIdx++
		}
	}

	var msg sharedTypes.EditorEvent
	{
		const addedLen = len(`{"i":"","n":"","j":1}`) + types.PublicIdLength
		const removedLen = len(`{"i":""}`) + types.PublicIdLength
		size := 1 + added*addedLen + namesSize + removed*removedLen +
			(added+removed)*1 - 1 + 1
		p := make([]byte, 0, size)
		p = append(p, '[')
		for i, rc := range rcs {
			if rc.IsJoin != 0 &&
				len(drop) > 0 &&
				drop[sort.SearchInts(drop, i)] == i {
				continue
			}
			p = rc.Append(p)
			p = append(p, ',')
		}
		p[len(p)-1] = ']'
		var source sharedTypes.PublicId
		if len(rcs) == 1 {
			source = rcs[0].PublicId
		}
		msg = sharedTypes.EditorEvent{
			Source:  source,
			Message: sharedTypes.ClientTrackingBatch,
			RoomId:  projectId,
			Payload: p,
		}
	}

	projectKey := getProjectKey(projectId)
	ctx := context.Background() // rely on connection timeout
	_, err := m.redisClient.TxPipelined(ctx, func(p redis.Pipeliner) error {
		if addedIdx > 0 {
			p.HSet(ctx, projectKey, hSet)
			p.Expire(ctx, projectKey, ProjectExpiry)
		}
		if removedIdx > 0 {
			p.HDelBulk(ctx, projectKey, hDel)
		}
		if _, err := m.c.PublishVia(ctx, p, &msg); err != nil {
			log.Printf("%s: publish room changes: %s", projectId, err)
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
